package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"alexGo-cloud/pkg/config"
)

// chdirRepoRoot 切到仓库根目录：LoadGlobalConfig 使用固定相对路径
// alexgo-server/configs/config.yaml（测试运行于 pkg/config 目录）。
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// 环境变量必须压过 config.yaml / 模块配置（K8s Secret 注入的前提）。
func TestLoadGlobalConfig_EnvOverridesBeatFile(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "envuser:envpass@tcp(127.0.0.1:3306)/envdb?charset=utf8mb4&parseTime=True&loc=Local")
	t.Setenv("JWT_SECRET", "env-jwt-secret")
	t.Setenv("REDIS_PASSWORD", "env-redis-pass")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if want := "envuser:envpass@tcp(127.0.0.1:3306)/envdb?charset=utf8mb4&parseTime=True&loc=Local"; cfg.Database.DSN != want {
		t.Errorf("DSN = %q, want env override %q", cfg.Database.DSN, want)
	}
	if cfg.System.JWTSecret != "env-jwt-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.System.JWTSecret, "env-jwt-secret")
	}
	if cfg.Redis.Password != "env-redis-pass" {
		t.Errorf("Redis.Password = %q, want %q", cfg.Redis.Password, "env-redis-pass")
	}
}

// env 为空时回退到 config 文件（本地开发体验不变）。
func TestLoadGlobalConfig_FallbackToFileWhenEnvEmpty(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("REDIS_PASSWORD", "")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.Database.DSN == "" {
		t.Error("DSN empty: config.yaml fallback broken")
	}
	if cfg.System.JWTSecret == "" {
		t.Error("JWTSecret empty: module config fallback broken")
	}
}

func TestCodegenUnitTestEnable_DefaultTrue(t *testing.T) {
	chdirRepoRoot(t)
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if !cfg.Codegen.UnitTestEnable {
		t.Error("codegen.unit_test_enable default = false, want true")
	}
}
