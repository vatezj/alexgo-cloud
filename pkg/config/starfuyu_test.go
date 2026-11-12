package config_test

import (
	"testing"

	"alexGo-cloud/pkg/config"
)

// starfuyu 段必须从 config.yaml 正确解码（mapstructure tag 与 yaml 键对齐，Task 1）。
// 缺省 cron / assets_dir 由 loader SetDefault 兜底，测试钉住最终读到的值。
func TestLoadGlobalConfig_StarfuyuSection(t *testing.T) {
	chdirRepoRoot(t)
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	if got := cfg.Starfuyu.Deepseek.Model; got != "deepseek-chat" {
		t.Errorf("starfuyu.deepseek.model = %q, want %q", got, "deepseek-chat")
	}
	if got := cfg.Starfuyu.Limits.BaziCalculateDaily; got != 5 {
		t.Errorf("starfuyu.limits.bazi_calculate_daily = %d, want 5", got)
	}
	if got := cfg.Starfuyu.Cron.FetchFortune; got != "10 0 * * *" {
		t.Errorf("starfuyu.cron.fetch_fortune = %q, want %q", got, "10 0 * * *")
	}
	if got := cfg.Starfuyu.Cron.GenerateImages; got != "20 0 * * *" {
		t.Errorf("starfuyu.cron.generate_images = %q, want %q", got, "20 0 * * *")
	}
	if got := cfg.Starfuyu.AssetsDir; got != "modules/starfuyu/assets" {
		t.Errorf("starfuyu.assets_dir = %q, want %q", got, "modules/starfuyu/assets")
	}
	// 密钥必须空串：仓库不落 goLucky 明文，由运维另填。
	if got := cfg.Starfuyu.Deepseek.APIKey; got != "" {
		t.Errorf("starfuyu.deepseek.api_key = %q, want empty", got)
	}
	if got := cfg.Starfuyu.WechatMp.AppSecret; got != "" {
		t.Errorf("starfuyu.wechat_mp.app_secret = %q, want empty", got)
	}
}
