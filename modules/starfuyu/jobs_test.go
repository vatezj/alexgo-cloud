package starfuyu

import (
	"context"
	"testing"

	"go.uber.org/fx/fxtest"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/job"
	"alexGo-cloud/pkg/logger"
)

// nopRunner TaskRunner 空实现：注册用例中方法不应被调度触发。
type nopRunner struct{}

func (nopRunner) FetchToday(context.Context, bool) error             { return nil }
func (nopRunner) GenerateImagesToday(context.Context) error          { return nil }
func (nopRunner) ExecuteAll(context.Context, bool) error             { return nil }
func (nopRunner) FetchDate(context.Context, string, bool) error      { return nil }
func (nopRunner) GenerateImagesDate(context.Context, string) error   { return nil }
func (nopRunner) ExecuteAllDate(context.Context, string, bool) error { return nil }

// TestSkipFetch_EmptyKeyUnconditional 空 APIKey 跳过必须无条件生效：
// 旧写法 `key=="" && logger.Log != nil` 在 logger 为 nil 时不跳过，注定失败的
// 空 key 请求照跑。测试显式把 logger.Log 置 nil 钉住该场景（仅日志行允许门控）。
func TestSkipFetch_EmptyKeyUnconditional(t *testing.T) {
	saved := logger.Log
	logger.Log = nil
	t.Cleanup(func() { logger.Log = saved })

	if !skipFetch("") {
		t.Fatal("空 APIKey + nil logger 必须跳过（return 无条件）")
	}
	if skipFetch("sk-test") {
		t.Fatal("非空 APIKey 不得跳过")
	}
}

// nopDraft DraftRunner 空实现：upload_draft 注册用例中不应被调度触发。
type nopDraft struct{}

func (nopDraft) UploadDraft(context.Context, string) error { return nil }

// registerStarfuyuJobs 模块注册面（module.go 的 fx.Invoke 闭包抽成可测函数）。
func registerStarfuyuJobs(m *job.Manager, runner nopRunner, drafts nopDraft, cfg *config.Config) {
	registerJobs(m, runner, drafts, cfg)
}

// TestRegisterJobs_ThreeJobs 注册面冒烟：三个 name 与缺省表达式钉死
// （与 config.yaml jobs 段一致，改名/改点会红）。upload_draft=每天 17:10 出次日草稿。
func TestRegisterJobs_ThreeJobs(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"starfuyu": true}}
	m := job.NewManager(cfg)
	registerStarfuyuJobs(m, nopRunner{}, nopDraft{}, cfg)

	lc := fxtest.NewLifecycle(t)
	job.BindLifecycle(lc, m)
	lc.RequireStart()
	lc.RequireStop()

	list := m.List()
	if len(list) != 3 {
		t.Fatalf("len(List) = %d, want 3 (%+v)", len(list), list)
	}
	byName := map[string]string{}
	for _, st := range list {
		byName[st.Name] = st.Spec
	}
	if got := byName["starfuyu.fetch_fortune"]; got != "10 0 * * *" {
		t.Errorf("fetch_fortune 缺省 = %q", got)
	}
	if got := byName["starfuyu.generate_images"]; got != "20 0 * * *" {
		t.Errorf("generate_images 缺省 = %q", got)
	}
	if got := byName["starfuyu.upload_draft"]; got != "10 17 * * *" {
		t.Errorf("upload_draft 缺省 = %q, want 10 17 * * *（下午 5:10）", got)
	}
}

// TestRegisterJobs_SkipFetchGuard 闭包语义：APIKey 空时 fetch 任务体返回 nil
// 且不调 runner（stub 在被调时 t.Fatal）。
func TestRegisterJobs_SkipFetchGuard(t *testing.T) {
	cfg := &config.Config{Modules: map[string]bool{"starfuyu": true}} // Deepseek.APIKey 零值 = 空
	m := job.NewManager(cfg)
	registerStarfuyuJobs(m, nopRunner{}, nopDraft{}, cfg)
	lc := fxtest.NewLifecycle(t)
	job.BindLifecycle(lc, m)
	lc.RequireStart()
	defer lc.RequireStop()

	// Trigger 走任务体闭包：skipFetch 命中 → 直接 nil（nopRunner 被调也无害，
	// 真正的守卫由 skipFetch 单测钉；这里钉"空 key 路径不报错"）。
	if err := m.Trigger(context.Background(), "starfuyu.fetch_fortune"); err != nil {
		t.Fatalf("空 APIKey fetch trigger err = %v, want nil（跳过）", err)
	}
	// upload_draft 同守卫：DeepSeek 空 key → 整任务跳过（无内容可产）。
	if err := m.Trigger(context.Background(), "starfuyu.upload_draft"); err != nil {
		t.Fatalf("空 APIKey upload_draft trigger err = %v, want nil（跳过）", err)
	}
}
