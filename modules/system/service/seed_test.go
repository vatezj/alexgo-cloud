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

// 迁移（fx.Invoke）先于 seed（OnStart）执行：fresh 库在 seed 观察时已非空——
// 旧门控 len==0 会误判"已播种"，11 个系统菜单永久缺失。以 /system 目录存在为准。
func TestHasSystemDir(t *testing.T) {
	if hasSystemDir(nil) {
		t.Error("empty must be false")
	}
	if hasSystemDir([]*model.Menu{{Path: "/dashboard", Type: "dir"}}) {
		t.Error("dashboard dir must not count (migration inserts it first)")
	}
	if !hasSystemDir([]*model.Menu{{Path: "/system", Type: "dir"}}) {
		t.Error("/system dir must count")
	}
	if hasSystemDir([]*model.Menu{{Path: "/system", Type: "menu"}}) {
		t.Error("type must be dir")
	}
	if !hasSystemDir([]*model.Menu{{Path: "/system", Type: "DIR"}}) {
		t.Error("type compare must be case-insensitive")
	}
	if hasSystemDir([]*model.Menu{nil, {Path: "/system/users", Type: "menu"}}) {
		t.Error("nil element must be skipped, not panic")
	}
}
