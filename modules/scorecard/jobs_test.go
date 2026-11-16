package scorecard

import (
	"testing"

	"go.uber.org/fx/fxtest"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/job"
)

// TestRegisterSweepJob 注册冒烟：超时关房清扫迁 pkg/job 后 name 与缺省 spec 钉死
// （与 config.yaml jobs 段一致，改名/改期会红）。
func TestRegisterSweepJob(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"scorecard": true}}
	m := job.NewManager(cfg)
	// svc 用 nil：只验注册与排程，不触发执行（@every 1m 测试窗口内可能跑，
	// 但 RoomService 是接口——nil 接口若被调会 panic 被 safeJob recover，
	// 不影响注册断言；正常不会跑，cron 首跑在 1 分钟后）。
	registerSweepJob(m, nil)

	lc := fxtest.NewLifecycle(t)
	job.BindLifecycle(lc, m)
	lc.RequireStart()
	lc.RequireStop()

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(list))
	}
	st := list[0]
	if st.Name != "scorecard.sweep_rooms" || st.Spec != "@every 1m" || !st.Enabled {
		t.Fatalf("Status = %+v, want scorecard.sweep_rooms/@every 1m/enabled", st)
	}
}
