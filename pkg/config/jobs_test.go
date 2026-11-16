package config_test

import (
	"testing"

	"alexGo-cloud/pkg/config"
)

// TestLoadGlobalConfig_JobsSection 钉 config.yaml 顶层 jobs: 段——四个 key 的值
// 必须与各模块 Register 的代码缺省一致（运维只改这里，代码缺省是兜底）。
func TestLoadGlobalConfig_JobsSection(t *testing.T) {
	chdirRepoRoot(t)
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig() error = %v", err)
	}
	want := map[string]string{
		"system.sweep_tokens":      "@every 1h",
		"scorecard.sweep_rooms":    "@every 1m",
		"starfuyu.fetch_fortune":   "10 0 * * *",
		"starfuyu.generate_images": "20 0 * * *",
	}
	flat := cfg.FlatJobs()
	for k, v := range want {
		if got := flat[k]; got != v {
			t.Errorf("FlatJobs()[%s] = %q, want %q", k, got, v)
		}
	}
}
