package errors

import (
	"errors"
	"testing"
)

func TestRegistryNoDuplicateAndAllHaveMsg(t *testing.T) {
	seen := map[Code]string{}
	for code, msg := range registry {
		if prev, ok := seen[code]; ok {
			t.Fatalf("duplicate code %d: %q vs %q", code, prev, msg)
		}
		if msg == "" {
			t.Fatalf("code %d has empty msg", code)
		}
		seen[code] = msg
	}
	// 导出常量必须都在注册表里
	for _, c := range []Code{
		CodeParamInvalid, CodeParamMissing,
		CodeUnauthorized, CodeTokenExpired, CodeForbidden,
		CodeUsernameTaken, CodeUserNotFound, CodeRoleTaken, CodeTenantDisabled, CodeCaptchaInvalid,
		CodeNoticeNotFound, CodeCodegenTableNotFound, CodeCodegenTableTaken,
		CodeMobileTaken,
		CodeInternal,
	} {
		if _, ok := registry[c]; !ok {
			t.Errorf("exported code %d missing from registry", c)
		}
	}
}

func TestNewAndMsgAndUnwrap(t *testing.T) {
	cause := errors.New("db down")
	e := New(CodeInternal, cause)
	if !errors.Is(e, cause) {
		t.Fatal("Unwrap must expose cause")
	}
	if e.Msg() != registry[CodeInternal] {
		t.Fatalf("Msg = %q", e.Msg())
	}
	if e.Error() == "" {
		t.Fatal("Error() empty")
	}
}

func TestIs(t *testing.T) {
	err := New(CodeUsernameTaken, nil)
	if !Is(err, CodeUsernameTaken) {
		t.Fatal("Is should match code")
	}
	if Is(err, CodeUserNotFound) {
		t.Fatal("Is should not match other code")
	}
	if Is(errors.New("x"), CodeUsernameTaken) {
		t.Fatal("Is on non-BizError must be false")
	}
}
