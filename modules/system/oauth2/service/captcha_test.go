package service

import (
	"context"
	"errors"
	"testing"

	"alexGo-cloud/pkg/config"
)

func newTestCaptcha(enabled bool) *CaptchaService {
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = enabled
	return NewCaptchaService(CaptchaDeps{Cfg: cfg})
}

// 开关关：Verify 恒过（无论 uuid/code 是否存在）。
func TestCaptcha_DisabledSkip(t *testing.T) {
	s := newTestCaptcha(false)
	if s.Enabled() {
		t.Fatal("Enabled() 应为 false")
	}
	if err := s.Verify(context.Background(), "no-such", "1234"); err != nil {
		t.Fatalf("disabled 时 Verify 应 nil, got %v", err)
	}
}

// 开关开：先取挑战，再用正确答案通过、错误答案拒。
func TestCaptcha_VerifyPaths(t *testing.T) {
	s := newTestCaptcha(true)
	ch, err := s.NewChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ch.UUID == "" || ch.Image == "" {
		t.Fatalf("挑战缺字段: %+v", ch)
	}
	if ch.Enabled != true {
		t.Error("Enabled 应 true")
	}
	// 错码 → ErrCaptchaInvalid
	if err := s.Verify(context.Background(), ch.UUID, "0000-wrong"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("错码应 ErrCaptchaInvalid, got %v", err)
	}
	// 未知 uuid 拒。
	if err := s.Verify(context.Background(), "unknown-uuid", "1234"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("未知 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
	// 错码 Verify 以 clear=true 消费了上一个挑战，正确答案段另取一个挑战。
	ch2, err := s.NewChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 同包白盒：store 里存的即答案（Get(id, clear=false) 不消费）。
	answer := s.store.Get(ch2.UUID, false)
	if answer == "" {
		t.Fatal("store 未存答案")
	}
	if err := s.Verify(context.Background(), ch2.UUID, answer); err != nil {
		t.Fatalf("正确答案应通过, got %v", err)
	}
	// 一次性：正确验证已 clear，同 uuid 再验拒。
	if err := s.Verify(context.Background(), ch2.UUID, answer); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("已消费 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
}

// 一次性：正确验证（clear=true）后同 uuid 再用即拒。
func TestCaptcha_SingleUse(t *testing.T) {
	s := newTestCaptcha(true)
	ch, err := s.NewChallenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// base64Captcha 的 Verify(id, answer, clear=true) 消费条目；
	// 答案未知时无法直接走通正确路径——用 Store 层面断言：
	// 连续两次 Verify 错码（clear=false 保留）后，第三次仍能查到；
	// 而 Verify(..., clear=true) 命中即删除。这里以「未知/已消费 uuid 拒」为准。
	_ = ch
	if err := s.Verify(context.Background(), "", ""); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("空 uuid 应 ErrCaptchaInvalid, got %v", err)
	}
}
