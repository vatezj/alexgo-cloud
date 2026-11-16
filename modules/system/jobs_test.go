package system

import (
	"testing"

	"go.uber.org/fx/fxtest"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/job"
	"alexGo-cloud/pkg/token"
)

// TestRegisterSweepJob 注册冒烟：token 清扫迁 pkg/job 后 name 与缺省 spec 钉死
// （与 config.yaml jobs 段一致，改名/改期会红）。
func TestRegisterSweepJob(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"system": true}}
	m := job.NewManager(cfg)
	// svc 用 nil：只验注册与排程，不触发执行（@every 1h 不会在测试窗口内跑）。
	registerSweepJob(m, (*token.Service)(nil))

	lc := fxtest.NewLifecycle(t)
	job.BindLifecycle(lc, m)
	lc.RequireStart()
	lc.RequireStop()

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(list))
	}
	st := list[0]
	if st.Name != "system.sweep_tokens" || st.Spec != "@every 1h" || !st.Enabled {
		t.Fatalf("Status = %+v, want system.sweep_tokens/@every 1h/enabled", st)
	}
}
