package config_test

import (
	"testing"

	"alexGo-cloud/pkg/config"
)

// taokouling 段必须从 config.yaml 正确解码（mapstructure tag 与 yaml 键对齐，Task 2）。
// 凭据（appkey/secret/apikey）提交为空串（裁定 R6-1），部署期由 env 注入——
// 故不钉任何凭据值，只钉非敏感结构字段与业务参数。
func TestLoadGlobalConfig_TaokoulingSection(t *testing.T) {
	chdirRepoRoot(t)
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if got := cfg.Taokouling.Zhetaoke.Sid; got != "21649" {
		t.Errorf("taokouling.zhetaoke.sid = %q, want %q", got, "21649")
	}
	if got := cfg.Taokouling.Zhetaoke.ParsePID; got != "mm_106702626_354300208_99225400058" {
		t.Errorf("taokouling.zhetaoke.parse_pid = %q, want %q", got, "mm_106702626_354300208_99225400058")
	}
	if got := cfg.Taokouling.Zhetaoke.BaseURL; got != "https://api.zhetaoke.com:10001" {
		t.Errorf("taokouling.zhetaoke.base_url = %q, want %q", got, "https://api.zhetaoke.com:10001")
	}
	if got := cfg.Taokouling.RebateRate; got != 0.05 {
		t.Errorf("taokouling.rebate_rate = %v, want 0.05", got)
	}
	if got := cfg.Taokouling.RateLimit.Limit; got != 10 {
		t.Errorf("taokouling.rate_limit.limit = %d, want 10", got)
	}
	if got := cfg.Taokouling.CacheTTLSeconds; got != 1800 {
		t.Errorf("taokouling.cache_ttl_seconds = %d, want 1800", got)
	}
	if cfg.Taokouling.FallbackSearchURL != "" {
		t.Errorf("taokouling.fallback_search_url 应默认空, got %q", cfg.Taokouling.FallbackSearchURL)
	}
}

// 三个凭据 env 非空时压过 yaml（applyEnvOverrides，DB_DSN/JWT_SECRET 同款，裁定 R6-3/4）。
func TestEnvOverrides_TaokoulingSecrets(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("TAOKOULING_ZHETAKE_APPKEY", "env-zhetaoke-appkey")
	t.Setenv("TAOKOULING_TOP_SECRET", "env-top-secret")
	t.Setenv("TAOKOULING_SHORTURL_APIKEY", "env-shorturl-apikey")

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if got := cfg.Taokouling.Zhetaoke.Appkey; got != "env-zhetaoke-appkey" {
		t.Errorf("Zhetaoke.Appkey = %q, want env override %q", got, "env-zhetaoke-appkey")
	}
	if got := cfg.Taokouling.Top.Secret; got != "env-top-secret" {
		t.Errorf("Top.Secret = %q, want env override %q", got, "env-top-secret")
	}
	if got := cfg.Taokouling.ShortURL.APIKey; got != "env-shorturl-apikey" {
		t.Errorf("ShortURL.APIKey = %q, want env override %q", got, "env-shorturl-apikey")
	}
}
