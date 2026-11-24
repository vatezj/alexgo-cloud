package service

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := NewRateLimiter(10, time.Minute, clk.Now)

	for i := 1; i <= 10; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("Allow #%d = false, want true (limit=10)", i)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("Allow #11 = true, want false (over limit)")
	}
}

func TestRateLimiter_WindowResets(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := NewRateLimiter(10, time.Minute, clk.Now)

	for i := 1; i <= 10; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("Allow #%d = false, want true", i)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("Allow #11 = true, want false (over limit)")
	}

	// 窗口过期 → 重置
	clk.advance(time.Minute + time.Nanosecond)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("Allow after window expiry = false, want true (window reset)")
	}
	// 重置后的窗口依旧限 limit 次
	for i := 2; i <= 10; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("Allow #%d in new window = false, want true", i)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("Allow #11 in new window = true, want false")
	}
}

func TestRateLimiter_IndependentIPs(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := NewRateLimiter(2, time.Minute, clk.Now)

	// ipA 用尽额度
	if !rl.Allow("ipA") || !rl.Allow("ipA") {
		t.Fatal("ipA first two Allows = false, want true")
	}
	if rl.Allow("ipA") {
		t.Fatal("ipA third Allow = true, want false (exhausted)")
	}
	// ipB 不受影响
	if !rl.Allow("ipB") {
		t.Fatal("ipB Allow = false, want true (independent of ipA)")
	}
	if !rl.Allow("ipB") {
		t.Fatal("ipB second Allow = false, want true")
	}
	if rl.Allow("ipB") {
		t.Fatal("ipB third Allow = true, want false (its own limit)")
	}
}
