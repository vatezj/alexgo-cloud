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

// DEPLOYMENT_MODE / SYSTEM_GRPC_ADDR 也必须被 env 压过（K8s 注入前提）。
// 此前只测了 DB_DSN/JWT_SECRET/REDIS_PASSWORD——覆盖矩阵缺口。
func TestLoadGlobalConfig_EnvOverridesDeploymentAndGRPC(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DEPLOYMENT_MODE", "micro")
	t.Setenv("SYSTEM_GRPC_ADDR", ":60051")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.Deployment.Mode != "micro" {
		t.Errorf("Deployment.Mode = %q, want micro", cfg.Deployment.Mode)
	}
	if cfg.SystemGRPCAddr != ":60051" {
		t.Errorf("SystemGRPCAddr = %q, want :60051", cfg.SystemGRPCAddr)
	}
}

// storage 段必须从 config.yaml 正确解码（mapstructure tag 与 yaml 键对齐，Task 2）。
func TestLoadGlobalConfig_StorageSection(t *testing.T) {
	chdirRepoRoot(t)
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if cfg.Storage.Root != "./data/uploads" {
		t.Errorf("storage.root = %q, want %q", cfg.Storage.Root, "./data/uploads")
	}
	if cfg.Storage.MaxSizeMB != 10 {
		t.Errorf("storage.max_size_mb = %d, want 10", cfg.Storage.MaxSizeMB)
	}
	if len(cfg.Storage.AllowedExts) != 0 {
		t.Errorf("storage.allowed_exts = %v, want empty (config.yaml 中已注释)", cfg.Storage.AllowedExts)
	}
}

func TestLoad_AuthCaptchaAndLoginPageDefaults(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DB_DSN", "")
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.Captcha.Enabled {
		t.Errorf("auth.captcha.enabled 默认应为 false, got true")
	}
	if cfg.Auth.LoginPage != "/auth/login" {
		t.Errorf("auth.login_page 默认 = %q, want /auth/login", cfg.Auth.LoginPage)
	}
}
