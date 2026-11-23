package errors_test

import (
	"testing"

	errs "alexGo-cloud/pkg/errors"
)

func TestTaoKoulingCodesRegistered(t *testing.T) {
	cases := map[errs.Code]string{
		errs.CodeTaoKoulingPasswordNotFound: "未识别到淘口令",
		errs.CodeTaoKoulingConfigMissing:    "淘口令服务配置缺失",
		errs.CodeTaoKoulingChannelMissing:   "未配置可用推广渠道",
		errs.CodeTaoKoulingRateLimited:      "请求过于频繁，请稍后再试",
		errs.CodeTaoKoulingParseFailed:      "淘口令解析失败",
	}
	for code, want := range cases {
		if got := errs.New(code, nil).Msg(); got != want {
			t.Errorf("Msg(%d) = %q, want %q", code, got, want)
		}
	}
}
