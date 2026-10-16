package service

import "testing"

// 文件存储三按钮与 permissionRoutes "infra:file" 组同源（与 TestSeedButtonPerms_MatchRouteKeys
// 同一防线）：漂移 = 死权限 403（按钮在但路由组不在）或路由失保护。
func TestSeedFileButtons_MatchRouteKey(t *testing.T) {
	if len(fileStorageButtons) != 3 {
		t.Fatalf("len(fileStorageButtons) = %d, want 3 (upload/download/delete)", len(fileStorageButtons))
	}
	routes, ok := permissionRoutes["infra:file"]
	if !ok {
		t.Fatal(`permissionRoutes 缺 "infra:file" 组——三按钮全是死权限`)
	}
	if len(routes) != 2 || routes[0] != "/api/admin/infra/file" || routes[1] != "/api/admin/infra/file/*" {
		t.Errorf("permissionRoutes[infra:file] = %v, want 集合路径 + /* 变体", routes)
	}
	want := map[string]bool{
		"infra:file:upload":   true,
		"infra:file:download": true,
		"infra:file:delete":   true,
	}
	for _, b := range fileStorageButtons {
		if !want[b.perm] {
			t.Errorf("意外按钮 perm = %q", b.perm)
		}
		// permPrefix 尺度必须命中路由组键（与既有三组按钮同判据）
		if permPrefix(b.perm) != "infra:file" {
			t.Errorf("permPrefix(%q) = %q, want infra:file", b.perm, permPrefix(b.perm))
		}
	}
}
