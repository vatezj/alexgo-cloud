package service

import (
	"testing"

	"alexGo-cloud/modules/system/model"
)

// hasPermPrefix 是种子按钮菜单 ensure-if-missing 的判据：必须按 permPrefix 尺度比对，
// 否则（用完整 permission 比对）永远不命中 → 每次启动重复插菜单（C3 修复的幂等前提）。
func TestHasPermPrefix_ScalesMatch(t *testing.T) {
	menus := []*model.Menu{
		{Permission: "system:auth:profile", Type: "button"},
		{Permission: "system:user:list", Type: "button"},
		nil, // 容忍 nil 节点
	}
	for prefix, want := range map[string]bool{
		"system:auth":   true, // 已存在（seed 的三组之一）
		"system:tenant": false, // 缺 → 应补
		"member:user":   false, // 缺 → 应补
		"system:user":   true,
		"system:menu":   false,
	} {
		if got := hasPermPrefix(menus, prefix); got != want {
			t.Errorf("hasPermPrefix(%q) = %v, want %v", prefix, got, want)
		}
	}
	// 空菜单集（全新库）：全部缺失 → 全部要补。
	if hasPermPrefix(nil, "system:auth") {
		t.Error("empty menus must report missing")
	}
}

// 三组按钮的 permPrefix 形态钉住（与 permissionRoutes 的键一致，防漂移）。
func TestSeedButtonPerms_MatchRouteKeys(t *testing.T) {
	for _, perm := range []string{"system:auth:profile", "system:tenant:manage", "member:user:manage"} {
		if _, ok := permissionRoutes[permPrefix(perm)]; !ok {
			t.Errorf("seed button %q → prefix %q has no permissionRoutes entry", perm, permPrefix(perm))
		}
	}
}
