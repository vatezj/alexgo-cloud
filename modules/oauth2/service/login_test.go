package service

import "testing"

func TestProtocolError_Format(t *testing.T) {
	e := &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号或密码错误"}
	if e.Error() != "400 invalid_grant: 账号或密码错误" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestSentinels_Distinct(t *testing.T) {
	// 五哨兵两两不等（错误映射矩阵依赖 errors.Is 区分）。
	all := []error{ErrInvalidCredentials, ErrAccountTypeUnsupported, ErrAuthenticatorMissing, ErrUserDisabled, ErrIssuance}
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if all[i] == all[j] {
				t.Errorf("哨兵 %d 与 %d 相同", i, j)
			}
		}
	}
}
