package service

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
)

// countingDriver 包一层 storage.Driver 统计 Save 次数（幂等跳过断言用）。
type countingDriver struct {
	storage.Driver
	saves int
}

func (d *countingDriver) Save(ctx context.Context, relPath string, r io.Reader) error {
	d.saves++
	return d.Driver.Save(ctx, relPath, r)
}

// imageFixture 内存 sqlite + 本地盘驱动 + 指定 assets 目录。
// assetsDir 传 "" 时用真实仓库资产 ../assets（含字体与背景图）。
type imageFixture struct {
	svc  ImageService
	drv  *countingDriver
	repo repository.FortuneRepository
}

func newImageFixture(t *testing.T, assetsDir string) *imageFixture {
	t.Helper()
	db := newTestDB(t)
	raw, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}
	drv := &countingDriver{Driver: raw}
	if assetsDir == "" {
		assetsDir = "../assets" // service 包 cwd → modules/starfuyu/assets
	}
	cfg := &config.Config{}
	cfg.Starfuyu.AssetsDir = assetsDir
	repo := repository.NewFortuneRepository(db)
	return &imageFixture{
		svc:  NewImageService(NewFortuneService(repo, &stubFetcher{}), drv, cfg),
		drv:  drv,
		repo: repo,
	}
}

func (f *imageFixture) seed(t *testing.T, rows []*model.Fortune) {
	t.Helper()
	if err := f.repo.UpsertBatch(context.Background(), rows); err != nil {
		t.Fatalf("预置运势行 err = %v", err)
	}
}

// TestEnsure_WritesJPEGAndSecondCallSkips 首次 Ensure 出 JPEG（key 正确、盘上 >0 字节）；
// 二次 Ensure 幂等跳过（Save 计数不增）。
func TestEnsure_WritesJPEGAndSecondCallSkips(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "图"))
	ctx := context.Background()

	key, err := f.svc.Ensure(ctx, date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure err = %v, want nil", err)
	}
	if want := "fortune/v2/2026-11-12/白羊座.jpg"; key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
	if f.drv.saves != 1 {
		t.Fatalf("首次 Ensure Save 次数 = %d, want 1", f.drv.saves)
	}

	rc, err := f.drv.Open(ctx, key)
	if err != nil {
		t.Fatalf("盘上文件 Open err = %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("读盘上文件 err = %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("盘上文件 0 字节")
	}
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		t.Fatalf("输出非 JPEG（缺 SOI 标记），前 4 字节 = % x", data[:min(4, len(data))])
	}

	key2, err := f.svc.Ensure(ctx, date, "白羊座")
	if err != nil {
		t.Fatalf("二次 Ensure err = %v, want nil", err)
	}
	if key2 != key {
		t.Fatalf("二次 Ensure key = %q, want %q", key2, key)
	}
	if f.drv.saves != 1 {
		t.Fatalf("二次 Ensure 应跳过重绘，Save 次数 = %d, want 1", f.drv.saves)
	}
}

// TestEnsure_MissingFortune30501 空库 Ensure → 30501、空 key。
func TestEnsure_MissingFortune30501(t *testing.T) {
	f := newImageFixture(t, "")

	key, err := f.svc.Ensure(context.Background(), "2026-11-12", "白羊座")
	if !errs.Is(err, errs.CodeStarfuyuFortuneNotFound) {
		t.Fatalf("空库 Ensure 应为 30501, got %v", err)
	}
	if key != "" {
		t.Fatalf("失败应返回空 key, got %q", key)
	}
}

// TestGenerateAll_PartialFailureContinues 12 缺 1（双鱼座）→ ok=11、
// 单张失败不中断（其余 11 张落盘）、err 聚合为 30502。
func TestGenerateAll_PartialFailureContinues(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "缺一")[:11]) // 缺双鱼座

	ok, err := f.svc.GenerateAll(context.Background(), date)
	if ok < 11 {
		t.Fatalf("ok = %d, want >= 11", ok)
	}
	if ok != 11 {
		t.Fatalf("ok = %d, want 11", ok)
	}
	if err == nil {
		t.Fatalf("单张失败应返回聚合错误, got nil")
	}
	if !errs.Is(err, errs.CodeStarfuyuImageFailed) {
		t.Fatalf("聚合错误码应为 30502, got %v", err)
	}
	if f.drv.saves != 11 {
		t.Fatalf("落盘张数 = %d, want 11（失败不中断）", f.drv.saves)
	}
}

// TestEnsure_FontMissing30502 assets 有背景图但无字体 → 30502（决定 19：禁静默 ASCII 回退）。
func TestEnsure_FontMissing30502(t *testing.T) {
	const date = "2026-11-12"
	// 临时 assets：复制真实背景图、不放字体。
	tmp := t.TempDir()
	bg, err := os.ReadFile(filepath.Join("..", "assets", "bgimage", "bg.png"))
	if err != nil {
		t.Fatalf("读真实背景图 err = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "bgimage"), 0o755); err != nil {
		t.Fatalf("Mkdir bgimage err = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "bgimage", "bg.png"), bg, 0o644); err != nil {
		t.Fatalf("写临时背景图 err = %v", err)
	}

	f := newImageFixture(t, tmp)
	f.seed(t, twelveRows(date, "无字体"))

	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if !errs.Is(err, errs.CodeStarfuyuImageFailed) {
		t.Fatalf("字体缺失应为 30502, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "字体") {
		t.Fatalf("错误应指明字体缺失, got %v", err)
	}
	if key != "" {
		t.Fatalf("失败应返回空 key, got %q", key)
	}
	if f.drv.saves != 0 {
		t.Fatalf("失败不得落盘, saves = %d", f.drv.saves)
	}
}
