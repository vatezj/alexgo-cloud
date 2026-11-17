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
	// cron 断言迁 jobs_test.go（顶层 jobs 段，FlatJobs 扁平读出）。
	if got := cfg.Starfuyu.AssetsDir; got != "modules/starfuyu/assets" {
		t.Errorf("starfuyu.assets_dir = %q, want %q", got, "modules/starfuyu/assets")
	}
	// 字体走配置、不放源码树：font_file 指向 data/fonts（运行时资源目录）。
	if got := cfg.Starfuyu.FontFile; got != "./data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf" {
		t.Errorf("starfuyu.font_file = %q, want %q", got, "./data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf")
	}
	// 底图同理：bg_file 指向 data/starfuyu（运行时目录，源码树外可替换）。
	if got := cfg.Starfuyu.BgFile; got != "./data/starfuyu/bg.png" {
		t.Errorf("starfuyu.bg_file = %q, want %q", got, "./data/starfuyu/bg.png")
	}
	// 角色抠图目录同理：cute_dir 指向 data/starfuyu/cute/cutout（12 张透明 PNG）。
	if got := cfg.Starfuyu.CuteDir; got != "./data/starfuyu/cute/cutout" {
		t.Errorf("starfuyu.cute_dir = %q, want %q", got, "./data/starfuyu/cute/cutout")
	}
	// 软文封面目录同理。
	if got := cfg.Starfuyu.CoverDir; got != "./data/starfuyu/cover" {
		t.Errorf("starfuyu.cover_dir = %q, want %q", got, "./data/starfuyu/cover")
	}
	// 密钥：2026-11-16 裁定改为允许本地明文（运营在本机 config.yaml 直填
	// deepseek/wechat_mp 密钥以便跑真实上传；原「必须空串」政策作废）。
	// ⚠ 提交 config.yaml 前必须先把 api_key/app_secret 清回空串——密钥不进 git。
	// 字段存在性由上方各段解码断言覆盖，此处不再钉空。
}
