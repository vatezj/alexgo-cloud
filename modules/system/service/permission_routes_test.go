package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 权限码 → 路由映射的双向守护：
// ① permissionRoutes 里每个键的前缀都能被 permPrefix 正确归类；
// ② 种子迁移里出现的每个 infra 权限码，其前缀必须已注册路由——
//    否则 casbin 无 policy，admin 403（Review Focus #4）。
func TestPermissionRoutes_CoverInfraSeed(t *testing.T) {
	// permPrefix 是前两段：infra:codegen:import → infra:codegen
	if got := permPrefix("infra:codegen:import"); got != "infra:codegen" {
		t.Errorf("permPrefix = %q", got)
	}
	if _, ok := permissionRoutes["infra:codegen"]; !ok {
		t.Fatal("permissionRoutes 缺 infra:codegen（种子菜单会 403）")
	}
	// 映射必须含根集合与通配子路径
	patterns := permissionRoutes["infra:codegen"]
	want := []string{"/api/admin/infra/codegen", "/api/admin/infra/codegen/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("infra:codegen 缺路由模式 %q，got %v", w, patterns)
		}
	}
}

// 扫种子 SQL：出现的 infra:codegen:* 权限码前缀必须都在 permissionRoutes。
func TestInfraSeed_PermsRegistered(t *testing.T) {
	// 本测试在 modules/system/service 下运行；迁移在 ../migrations
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'((infra|system):[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "codegen_menus") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(migPath, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range permRe.FindAllStringSubmatch(string(b), -1) {
			checked++
			if _, ok := permissionRoutes[permPrefix(m[1])]; !ok {
				t.Errorf("%s 中权限码 %q 前缀 %q 未注册路由", e.Name(), m[1], permPrefix(m[1]))
			}
		}
	}
	if checked == 0 {
		t.Error("种子迁移未扫到任何权限码——正则或 SQL 结构变了，测试失效")
	}
}
