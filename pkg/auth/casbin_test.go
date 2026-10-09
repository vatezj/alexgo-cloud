package auth_test

import (
	"testing"

	"alexGo-cloud/pkg/auth"
)

// C1：用户 sub 必须带 user_type 维度——member 昵称与管理员用户名同名时永不同 sub
//（否则 casbin g(x,x) 恒等 → 成员继承管理员策略）。
func TestUserSub_TypeDimension(t *testing.T) {
	admin := auth.UserSub(2, 1, "admin", 9)
	member := auth.UserSub(2, 2, "admin", 9)
	if admin == member {
		t.Fatalf("member sub collides with admin sub: %q", admin)
	}
	if want := "2:1:admin"; admin != want {
		t.Errorf("admin sub = %q, want %q", admin, want)
	}
	// member 形态钉死：{tid}:{ut}:{nickname} → 2:2:{nick}。
	if got, want := auth.UserSub(2, 2, "alice", 9), "2:2:alice"; got != want {
		t.Errorf("member sub = %q, want %q", got, want)
	}
	// 空 name 退化为 user id，同样保留 user_type 维度。
	if got, want := auth.UserSub(2, 2, "", 9), "2:2:9"; got != want {
		t.Errorf("empty-name sub = %q, want %q", got, want)
	}
}
