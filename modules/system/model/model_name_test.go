package model

import "testing"

// users 表已重命名为 system_users，TableName 必须跟随（否则迁移后 GORM 全部打到不存在的表）。
func TestUserTableName(t *testing.T) {
	if got := (User{}).TableName(); got != "system_users" {
		t.Errorf("User.TableName() = %q, want system_users", got)
	}
}
