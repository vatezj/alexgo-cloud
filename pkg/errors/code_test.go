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
		CodeFileNotFound, CodeFileTypeNotAllowed, CodeFileTooLarge,
		CodeExportTooManyRows,
		CodeSocialAppNotConfigured,       // 30301
		CodeSocialExchangeFailed,         // 30302
		CodeSocialIdentityBound,          // 30303
		CodeSocialMobileBound,            // 30304
		CodeSocialMergeRefused,           // 30305
		CodeSocialUnbindLast,             // 30306
		CodeFishgameFishNotFound,         // 30401
		CodeFishgameFishReferenced,       // 30402
		CodeFishgameFishKingInvalid,      // 30403
		CodeFishgameConfigNotFound,       // 30404
		CodeStarfuyuFortuneNotFound,      // 30501
		CodeStarfuyuImageFailed,          // 30502
		CodeStarfuyuLimitExceeded,        // 30503
		CodeStarfuyuBaziNotFound,         // 30504
		CodeStarfuyuConstellationMissing, // 30505
		CodeStarfuyuFetchFailed,          // 30506
		CodeStarfuyuMpUploadFailed,       // 30507
		CodeInternal,
	} {
		if _, ok := registry[c]; !ok {
			t.Errorf("exported code %d missing from registry", c)
		}
	}
	// starfuyu 305xx 必须齐且文案非空（spec §7）。
	for _, c := range []Code{30501, 30502, 30503, 30504, 30505, 30506, 30507} {
		if registry[c] == "" {
			t.Errorf("code %d missing registry message", c)
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
