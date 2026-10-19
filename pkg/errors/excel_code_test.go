package errors

import "testing"

// 导出错误码文案回归：文案写死中文、码值固定（30013 → HTTP 400 由 resp.statusFor
// 的 3xxxx 分支自动映射，勿改 resp）。模式照 file_code_test.go。
func TestExportErrorCodes(t *testing.T) {
	be := New(CodeExportTooManyRows, nil)
	if got := be.Msg(); got != "导出行数超过上限" {
		t.Errorf("New(30013).Msg() = %q, want %q", got, "导出行数超过上限")
	}
	if !Is(be, CodeExportTooManyRows) {
		t.Error("Is(New(30013), CodeExportTooManyRows) must be true")
	}
}
