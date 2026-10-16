package errors

import "testing"

// 文件存储三码文案回归：文案写死中文、码值固定（30010-30012 与 spec §7 一致）。
func TestFileErrorCodes(t *testing.T) {
	cases := []struct {
		code Code
		want string
	}{
		{CodeFileNotFound, "文件不存在"},
		{CodeFileTypeNotAllowed, "文件类型不允许"},
		{CodeFileTooLarge, "文件超过大小限制"},
	}
	for _, tc := range cases {
		be := New(tc.code, nil)
		if got := be.Msg(); got != tc.want {
			t.Errorf("New(%d).Msg() = %q, want %q", tc.code, got, tc.want)
		}
	}
}
