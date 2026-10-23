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
//
//	否则 casbin 无 policy，admin 403（Review Focus #4）。
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

// Task 13：oauth2 client 管理路由前缀必须注册——否则 Task 16 菜单种子产生的
// oauth2:client 权限在 Casbin 无 policy，admin 打 client 管理页 403。
func TestPermissionRoutes_Oauth2ClientRegistered(t *testing.T) {
	patterns, ok := permissionRoutes["oauth2:client"]
	if !ok {
		t.Fatal("permissionRoutes 缺 oauth2:client（client 管理页会 403）")
	}
	want := []string{"/api/admin/oauth2/clients", "/api/admin/oauth2/clients/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("oauth2:client 缺路由模式 %q, got %v", w, patterns)
		}
	}
}

// 扫 oauth2 菜单种子 SQL：出现的 oauth2:client* 权限码前缀必须都已注册路由——
// 否则 Task 16 种子产生的页面/按钮在 Casbin 无 policy，admin 403。
// 正则要求冒号：目录权限字面量 'oauth2' 与 LIKE 'oauth2:%' 天然排除
// （前者无冒号不匹配；后者冒号后是 %，不在 [a-z0-9_:] 字符类内）。
func TestOauth2Seed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(oauth2:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "oauth2_client_menu") {
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
		t.Error("oauth2 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}

// 终审 I1：plan Review Focus #5 假设「既有机制自动兜底新键」——对 system:online
// 为假（既有两处扫描只认 codegen/oauth2 文件名）。钉死三链路的路由侧：
// ① permissionRoutes["system:online"] 两模式齐全；② online_user_menus 迁移里
// 出现的每个 system:online:* 权限码，其前缀均已注册——码与键漂移即红。
func TestPermissionRoutes_SystemOnlineRegistered(t *testing.T) {
	patterns, ok := permissionRoutes["system:online"]
	if !ok {
		t.Fatal(`permissionRoutes 缺 "system:online"（在线用户页会 403）`)
	}
	want := []string{"/api/admin/system/online", "/api/admin/system/online/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("system:online 缺路由模式 %q, got %v", w, patterns)
		}
	}
}

// 扫在线用户菜单种子 SQL：出现的 system:online:* 权限码前缀必须都已注册路由——
// 否则种子产生的页面/按钮在 Casbin 无 policy，admin 403。
// 正则要求 system:online: 前缀：LIKE 'system:online%' 与目录字面量天然排除。
func TestOnlineSeed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(system:online:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "online_user_menus") {
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
		t.Error("在线用户菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}

// Task 9：social:app 渠道配置路由前缀必须注册——否则渠道配置页在 Casbin
// 无 policy，admin 403（brief 用例逐字）。
func TestPermissionRoutes_SocialAppRegistered(t *testing.T) {
	patterns, ok := permissionRoutes["social:app"]
	if !ok {
		t.Fatal(`permissionRoutes 缺 "social:app"（渠道配置页会 403）`)
	}
	want := []string{"/api/admin/social/apps", "/api/admin/social/apps/*"}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("social:app 缺路由模式 %q, got %v", w, patterns)
		}
	}
}

// 扫渠道配置菜单种子 SQL：出现的 social:* 权限码前缀必须都已注册路由——
// 否则种子产生的页面/按钮在 Casbin 无 policy，admin 403。
// 正则要求冒号：目录权限字面量 'social' 与 LIKE 'social:%' 天然排除
// （前者无冒号不匹配；后者冒号后是 %，不在 [a-z0-9_:] 字符类内）。
func TestSocialSeed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(social:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "social_app_menu") {
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
		t.Error("social 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}

// ---- fishgame（终审 Important#5：Task 9 Step 3 期望的 5 前缀覆盖守护） ----

func TestPermissionRoutes_FishgameRegistered(t *testing.T) {
	for code, want := range map[string][]string{
		"fishgame:fish":   {"/api/admin/fishgame/fishes", "/api/admin/fishgame/fishes/*"},
		"fishgame:config": {"/api/admin/fishgame/configs", "/api/admin/fishgame/configs/*"},
		"fishgame:event":  {"/api/admin/fishgame/events", "/api/admin/fishgame/events/*"},
		"fishgame:player": {"/api/admin/fishgame/players", "/api/admin/fishgame/players/*"},
		"fishgame:stats":  {"/api/admin/fishgame/stats"},
	} {
		patterns, ok := permissionRoutes[code]
		if !ok {
			t.Fatalf(`permissionRoutes 缺 %q（fishgame 管理页会 403）`, code)
		}
		for _, w := range want {
			found := false
			for _, p := range patterns {
				if p == w {
					found = true
				}
			}
			if !found {
				t.Errorf("%s 缺路由模式 %q, got %v", code, w, patterns)
			}
		}
	}
}

// 扫 fishgame 菜单种子 SQL：出现的 fishgame:* 权限码前缀必须都已注册路由。
func TestFishgameSeed_PermsRegistered(t *testing.T) {
	migPath := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(migPath)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	permRe := regexp.MustCompile(`'(fishgame:[a-z0-9_:]+)'`)
	checked := 0
	for _, e := range entries {
		if !strings.Contains(e.Name(), "fishgame_menus") {
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
		t.Error("fishgame 菜单迁移未扫到任何权限码——文件名或正则失配，测试失效")
	}
}
