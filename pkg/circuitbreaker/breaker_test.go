package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func fail() error { return errors.New("down") }

// 连续失败达 threshold → Open，后续请求快速失败且不再执行 fn。
func TestDo_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	for i := 0; i < 3; i++ {
		if err := cb.Do(fail); err == nil {
			t.Fatalf("round %d: want error from fn", i)
		}
	}
	if cb.state != Open {
		t.Fatalf("state = %v, want Open", cb.state)
	}

	executed := false
	err := cb.Do(func() error { executed = true; return nil })
	if !errors.Is(err, ErrOpen) {
		t.Errorf("err = %v, want ErrOpen", err)
	}
	if executed {
		t.Error("fn must not execute while breaker is Open")
	}
}

// 成功调用会把失败计数清零（不会累积到阈值）。
func TestDo_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	_ = cb.Do(fail)
	_ = cb.Do(fail)
	if err := cb.Do(func() error { return nil }); err != nil {
		t.Fatalf("success Do() error = %v", err)
	}
	if cb.failures != 0 {
		t.Errorf("failures = %d, want 0 after success", cb.failures)
	}
	// 再失败 2 次仍不应打开（第 3 次成功已重置）。
	_ = cb.Do(fail)
	_ = cb.Do(fail)
	if cb.state != Closed {
		t.Errorf("state = %v, want Closed", cb.state)
	}
}

// Open 期拒绝 → openFor 到期进入 HalfOpen 放行探测 → 探测成功回到 Closed。
func TestDo_OpenThenHalfOpenRecovers(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	if err := cb.Do(fail); err == nil {
		t.Fatal("setup: fail() should return error")
	}
	if cb.state != Open {
		t.Fatalf("setup: state = %v, want Open", cb.state)
	}

	// 未到期：拒绝。
	if err := cb.Do(func() error { return nil }); !errors.Is(err, ErrOpen) {
		t.Fatalf("before openFor elapsed: err = %v, want ErrOpen", err)
	}

	time.Sleep(60 * time.Millisecond)

	// 到期：放行探测，成功 → Closed。
	if err := cb.Do(func() error { return nil }); err != nil {
		t.Errorf("half-open probe err = %v, want nil", err)
	}
	if cb.state != Closed {
		t.Errorf("state = %v, want Closed after successful probe", cb.state)
	}
}

// HalfOpen 探测失败 → 立即回到 Open。
func TestDo_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	_ = cb.Do(fail)
	time.Sleep(60 * time.Millisecond)
	if err := cb.Do(fail); err == nil {
		t.Fatal("probe should return fn error")
	}
	if cb.state != Open {
		t.Errorf("state = %v, want Open after failed probe", cb.state)
	}
}

// nil 接收者直接执行（与 Allow 一样的 fail-open 约定）。
func TestDo_NilReceiver(t *testing.T) {
	var cb *CircuitBreaker
	executed := false
	if err := cb.Do(func() error { executed = true; return nil }); err != nil {
		t.Fatalf("nil cb.Do() error = %v", err)
	}
	if !executed {
		t.Error("fn must execute on nil receiver")
	}
}

// 非法参数回退默认值，不 panic。
func TestNewCircuitBreaker_Defaults(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb.threshold != 10 || cb.openFor != 30*time.Second {
		t.Errorf("defaults = (%d, %v), want (10, 30s)", cb.threshold, cb.openFor)
	}
}

// 直接驱动 allow()/after()，钉住 Open→HalfOpen 转换与 halfInUse 单探测守卫
//（若回归跳过 HalfOpen 直转 Closed，本测试必须变红）。
func TestAllow_HalfOpenTransitionAndProbeGuard(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	_ = cb.Do(fail) // → Open
	if cb.state != Open {
		t.Fatalf("state = %v, want Open", cb.state)
	}
	time.Sleep(60 * time.Millisecond)

	// 到期后第一次 allow：放行探测并占用 halfInUse。
	if !cb.allow() {
		t.Fatal("after openFor elapsed, allow() must admit a probe")
	}
	if cb.state != HalfOpen {
		t.Errorf("state = %v, want HalfOpen (regression: expired Open must enter HalfOpen, not Closed)", cb.state)
	}
	if !cb.halfInUse {
		t.Error("halfInUse must be set while probe is in flight")
	}

	// 探测占用期间，第二个请求必须被拒（单探测守卫）。
	if cb.allow() {
		t.Error("second request must be rejected while half-open probe in flight")
	}

	// 探测成功 → Closed + 计数清零。
	cb.after(true)
	if cb.state != Closed {
		t.Errorf("state = %v, want Closed after successful probe", cb.state)
	}
	if cb.failures != 0 {
		t.Errorf("failures = %d, want 0", cb.failures)
	}

	// Closed 下放行正常请求（不占用探测位）。
	if !cb.allow() {
		t.Error("allow() must admit requests in Closed state")
	}
	if cb.halfInUse {
		t.Error("halfInUse must not be set in Closed state")
	}
}
