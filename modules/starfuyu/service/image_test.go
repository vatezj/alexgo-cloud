package service

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fogleman/gg"

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
	fontFile, bgFile := "", ""
	if assetsDir == "" {
		assetsDir = "../assets" // service 包 cwd → modules/starfuyu/assets（仅 legacy 回退）
		// 字体/底图在源码树外（配置；相对 service 包 cwd → 仓库根 data/）。
		fontFile = "../../../data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf"
		bgFile = "../../../data/starfuyu/bg.png"
	}
	return newImageFixtureCfg(t, assetsDir, fontFile, bgFile)
}

// newImageFixtureCfg 显式指定 assetsDir/fontFile/bgFile 的底层夹具
// （FontFile/BgFile 来源测试用；显式 assetsDir 且 fontFile/bgFile 为空 → 走 legacy 回退）。
func newImageFixtureCfg(t *testing.T, assetsDir, fontFile, bgFile string) *imageFixture {
	t.Helper()
	db := newTestDB(t)
	raw, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDriver: %v", err)
	}
	drv := &countingDriver{Driver: raw}
	cfg := &config.Config{}
	cfg.Starfuyu.AssetsDir = assetsDir
	cfg.Starfuyu.FontFile = fontFile
	cfg.Starfuyu.BgFile = bgFile
	// 角色抠图在仓库根 data/（相对 service 包 cwd）——所有渲染测试共用真实 12 张产物。
	cfg.Starfuyu.CuteDir = "../../../data/starfuyu/cute/cutout"
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
	if want := "fortune/v3/2026-11-12/白羊座.jpg"; key != want {
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

// TestEnsure_FontFileConfigured assets 无字体、font_file 指向源码树外真字体 → 出图：
// 字体来源=font_file 配置（不走 assets/fonts glob）。
func TestEnsure_FontFileConfigured(t *testing.T) {
	const date = "2026-11-12"
	// 临时 assets：只放背景图、不放字体。
	tmp := t.TempDir()
	bg, err := os.ReadFile(filepath.Join("..", "..", "..", "data", "starfuyu", "bg.png"))
	if err != nil {
		t.Fatalf("读真实背景图 err = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "bgimage"), 0o755); err != nil {
		t.Fatalf("Mkdir bgimage err = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "bgimage", "bg.png"), bg, 0o644); err != nil {
		t.Fatalf("写临时背景图 err = %v", err)
	}

	f := newImageFixtureCfg(t, tmp, "../../../data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf", "")
	f.seed(t, twelveRows(date, "字体配置"))

	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if err != nil {
		t.Fatalf("FontFile 出图 err = %v, want nil（字体应来自 font_file）", err)
	}
	if key == "" || f.drv.saves != 1 {
		t.Fatalf("key = %q, saves = %d, want 非空且落盘 1", key, f.drv.saves)
	}
}

// TestEnsure_FontFilePathMissing30502 font_file 指向不存在文件 → 30502 指明 font_file，
// 不静默回退 glob（配置错了必须炸，决定 19 同精神）。
func TestEnsure_FontFilePathMissing30502(t *testing.T) {
	const date = "2026-11-12"
	tmp := t.TempDir()
	bg, err := os.ReadFile(filepath.Join("..", "..", "..", "data", "starfuyu", "bg.png"))
	if err != nil {
		t.Fatalf("读真实背景图 err = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "bgimage"), 0o755); err != nil {
		t.Fatalf("Mkdir bgimage err = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "bgimage", "bg.png"), bg, 0o644); err != nil {
		t.Fatalf("写临时背景图 err = %v", err)
	}

	f := newImageFixtureCfg(t, tmp, filepath.Join(tmp, "nope.ttf"), "")
	f.seed(t, twelveRows(date, "缺字体文件"))

	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if !errs.Is(err, errs.CodeStarfuyuImageFailed) {
		t.Fatalf("font_file 缺失应为 30502, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "font_file") {
		t.Fatalf("错误应指明 font_file 配置, got %v", err)
	}
	if key != "" || f.drv.saves != 0 {
		t.Fatalf("失败不得落盘: key = %q, saves = %d", key, f.drv.saves)
	}
}

// TestRender_BgFileMissing30502 bg_file 指向不存在底图 → 30502 指明 bg_file，
// 不静默回退 assetsDir（配置错了必须炸，与 font_file 同精神）。
func TestRender_BgFileMissing30502(t *testing.T) {
	const date = "2026-11-12"
	tmp := t.TempDir() // 空 assets：legacy 回退也无底图，唯一起作用的是 bg_file
	f := newImageFixtureCfg(t, tmp,
		"../../../data/fonts/AlibabaPuHuiTi-3-85-Bold.ttf",
		filepath.Join(tmp, "nope.png"))
	f.seed(t, twelveRows(date, "缺底图"))

	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if !errs.Is(err, errs.CodeStarfuyuImageFailed) {
		t.Fatalf("bg_file 缺失应为 30502, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "bg_file") {
		t.Fatalf("错误应指明 bg_file 配置, got %v", err)
	}
	if key != "" || f.drv.saves != 0 {
		t.Fatalf("失败不得落盘: key = %q, saves = %d", key, f.drv.saves)
	}
}

// TestEnsure_FontMissing30502 assets 有背景图但无字体 → 30502（决定 19：禁静默 ASCII 回退）。
func TestEnsure_FontMissing30502(t *testing.T) {
	const date = "2026-11-12"
	// 临时 assets：复制真实背景图、不放字体。
	tmp := t.TempDir()
	bg, err := os.ReadFile(filepath.Join("..", "..", "..", "data", "starfuyu", "bg.png"))
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

// decodeRendered 从 fixture 盘上读出 key 对应 JPEG 并解码（像素断言用）。
func decodeRendered(t *testing.T, f *imageFixture, ctx context.Context, key string) image.Image {
	t.Helper()
	rc, err := f.drv.Open(ctx, key)
	if err != nil {
		t.Fatalf("打开盘上 %q err = %v", key, err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("读盘上 %q err = %v", key, err)
	}
	im, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码 %q JPEG err = %v", key, err)
	}
	return im
}

// loadShareBG 底图原图（像素比较基准）。
func loadShareBG(t *testing.T) image.Image {
	t.Helper()
	bg, err := gg.LoadImage(filepath.Join("..", "..", "..", "data", "starfuyu", "bg.png"))
	if err != nil {
		t.Fatalf("加载底图 err = %v", err)
	}
	return bg
}

// absF 浮点绝对值。
func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// regionMeanDiff 两图同 rect 逐通道差的均值（JPEG q90 噪声容差断言用）。
func regionMeanDiff(a, b image.Image, r slot) float64 {
	var sum, n float64
	for y := int(r.y); y < int(r.y+r.h); y++ {
		for x := int(r.x); x < int(r.x+r.w); x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := b.At(x, y).RGBA()
			sum += absF(float64(r1>>8)-float64(r2>>8)) + absF(float64(g1>>8)-float64(g2>>8)) + absF(float64(b1>>8)-float64(b2>>8))
			n += 3
		}
	}
	return sum / n
}

// regionMaxDiff 两图同 rect 逐通道差的最大值（有值必画断言用）。
func regionMaxDiff(a, b image.Image, r slot) float64 {
	m := 0.0
	for y := int(r.y); y < int(r.y+r.h); y++ {
		for x := int(r.x); x < int(r.x+r.w); x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := b.At(x, y).RGBA()
			for _, d := range [...]float64{
				absF(float64(r1>>8) - float64(r2>>8)),
				absF(float64(g1>>8) - float64(g2>>8)),
				absF(float64(b1>>8) - float64(b2>>8)),
			} {
				if d > m {
					m = d
				}
			}
		}
	}
	return m
}

// TestRender_EmptyShareFields 空值槽位：留白不崩，且槽位区域像素≈底图（未画残留符号）；
// 有值对照行同区域必须有绘制（像素差 > 20）。
func TestRender_EmptyShareFields(t *testing.T) {
	const date = "2026-11-12"
	ctx := context.Background()

	// 空值行：逐行清空分享卡新字段，Summary 也置空（twelveRows 默认满值，须显式清）。
	f := newImageFixture(t, "")
	rows := twelveRows(date, "空")
	for _, row := range rows {
		row.Summary = ""
		row.MatchConstellation = ""
		row.Advice = ""
		row.Avoid = ""
		row.ReunionTip, row.SingleTip, row.JobTip = "", "", ""
		row.ReunionScore, row.SingleScore, row.JobScore = 0, 0, 0
		row.DailyTodos = nil
		row.Slogan = ""
	}
	f.seed(t, rows)

	key, err := f.svc.Ensure(ctx, date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure(空值) err = %v, want nil（空字段不崩）", err)
	}
	empty := decodeRendered(t, f, ctx, key)
	bg := loadShareBG(t)

	if d := regionMeanDiff(empty, bg, shareCardLayout.Summary); d >= 6 {
		t.Fatalf("空 Summary 槽区域平均像素差 = %.2f, want < 6（留白不画）", d)
	}

	// 对照行：Summary 非空，同区域必须有绘制。
	f2 := newImageFixture(t, "")
	f2.seed(t, twelveRows(date, "有")) // Summary = "有总评"
	key2, err := f2.svc.Ensure(ctx, date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure(有值) err = %v, want nil", err)
	}
	full := decodeRendered(t, f2, ctx, key2)
	if d := regionMaxDiff(full, bg, shareCardLayout.Summary); d <= 20 {
		t.Fatalf("有值 Summary 槽区域最大像素差 = %.2f, want > 20（有值必画）", d)
	}
}

// TestRender_FullShareFields 满值出图不崩 + 关键槽区域确有绘制 + 进度条填充段覆盖中点。
func TestRender_FullShareFields(t *testing.T) {
	const date = "2026-11-12"
	ctx := context.Background()
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "满"))

	ok, err := f.svc.GenerateAll(ctx, date)
	if err != nil {
		t.Fatalf("GenerateAll err = %v, want nil", err)
	}
	if ok != 12 {
		t.Fatalf("GenerateAll ok = %d, want 12", ok)
	}

	img := decodeRendered(t, f, ctx, "fortune/v3/"+date+"/白羊座.jpg")
	bg := loadShareBG(t)

	if d := regionMaxDiff(img, bg, shareCardLayout.Advice); d <= 20 {
		t.Fatalf("Advice 槽区域最大像素差 = %.2f, want > 20（满值必画）", d)
	}

	// 底部粉条口号（白字）：Slogan 槽区域必须有绘制。
	if d := regionMaxDiff(img, bg, shareCardLayout.Slogan); d <= 20 {
		t.Fatalf("Slogan 槽区域最大像素差 = %.2f, want > 20（满值必画）", d)
	}

	// BarReunion 中点须被填充段覆盖（ReunionScore=66 > 50）：颜色与底图不同。
	b := shareCardLayout.BarReunion
	cx, cy := int(b.x+b.w/2), int(b.y+b.h/2)
	r1, g1, b1, _ := img.At(cx, cy).RGBA()
	r2, g2, b2, _ := bg.At(cx, cy).RGBA()
	diff := (absF(float64(r1>>8)-float64(r2>>8)) + absF(float64(g1>>8)-float64(g2>>8)) + absF(float64(b1>>8)-float64(b2>>8))) / 3
	if diff <= 20 {
		t.Fatalf("BarReunion 中点(%d,%d) 与底图平均色差 = %.0f, want > 20（填充段覆盖中点）", cx, cy, diff)
	}
}

// TestRender_TitleDrawn 顶部标题槽：星座名必画，区域与底图最大像素差 > 20。
func TestRender_TitleDrawn(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "题"))
	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure err = %v, want nil", err)
	}
	img := decodeRendered(t, f, context.Background(), key)
	bg := loadShareBG(t)
	if d := regionMaxDiff(img, bg, shareCardLayout.Title); d <= 20 {
		t.Fatalf("Title 槽区域最大像素差 = %.2f, want > 20（标题必画）", d)
	}
}

// TestConstellationSlug 12 星座→slug 硬表全量覆盖、与 ConstellationList 对齐、slug 无重复。
func TestConstellationSlug(t *testing.T) {
	want := map[string]string{
		"白羊座": "aries", "金牛座": "taurus", "双子座": "gemini", "巨蟹座": "cancer",
		"狮子座": "leo", "处女座": "virgo", "天秤座": "libra", "天蝎座": "scorpio",
		"射手座": "sagittarius", "摩羯座": "capricorn", "水瓶座": "aquarius", "双鱼座": "pisces",
	}
	if len(ConstellationList) != 12 {
		t.Fatalf("ConstellationList 长度 = %d, want 12", len(ConstellationList))
	}
	seen := make(map[string]bool, len(want))
	for _, c := range ConstellationList {
		got, ok := constellationSlug(c)
		if !ok {
			t.Fatalf("constellationSlug(%q) 未命中", c)
		}
		if want[c] != got {
			t.Fatalf("constellationSlug(%q) = %q, want %q", c, got, want[c])
		}
		if seen[got] {
			t.Fatalf("slug %q 重复", got)
		}
		seen[got] = true
	}
	if len(seen) != 12 {
		t.Fatalf("slug 覆盖 %d 条, want 12", len(seen))
	}
	if _, ok := constellationSlug("不存在座"); ok {
		t.Fatalf("非法星座应未命中")
	}
}

// TestRender_MascotDrawn 角色槽必画：Mascot 区域与底图最大像素差 > 20。
func TestRender_MascotDrawn(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.seed(t, twelveRows(date, "角"))
	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure err = %v, want nil", err)
	}
	img := decodeRendered(t, f, context.Background(), key)
	bg := loadShareBG(t)
	if d := regionMaxDiff(img, bg, shareCardLayout.Mascot); d <= 20 {
		t.Fatalf("Mascot 槽区域最大像素差 = %.2f, want > 20（角色必画）", d)
	}
}

// TestRender_CuteFileMissing30502 cute_dir 指向空目录 → 30502 指明 cute、不落盘
// （配置错了必须炸，与 font_file/bg_file 同精神）。
func TestRender_CuteFileMissing30502(t *testing.T) {
	const date = "2026-11-12"
	f := newImageFixture(t, "")
	f.svc.(*imageService).cuteDir = t.TempDir() // 同包测试直改字段，免动夹具签名
	f.seed(t, twelveRows(date, "缺角色"))

	key, err := f.svc.Ensure(context.Background(), date, "白羊座")
	if !errs.Is(err, errs.CodeStarfuyuImageFailed) {
		t.Fatalf("cute 文件缺失应为 30502, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "cute") {
		t.Fatalf("错误应指明 cute 素材, got %v", err)
	}
	if key != "" || f.drv.saves != 0 {
		t.Fatalf("失败不得落盘: key = %q, saves = %d", key, f.drv.saves)
	}
}

// TestRenderSampleForVisual 视觉验收采样：满值白羊座出样图到 /tmp，供人工对照底图检查。
// 分享卡字段用顶格字数内容（对照 prompt 上限），以暴露换行/截断 … 效果。
// go test -short 跳过（CI 不落盘）；本地 -run 显式跑。
func TestRenderSampleForVisual(t *testing.T) {
	if testing.Short() {
		t.Skip("视觉采样：-short 跳过")
	}
	const date = "2026-11-12"
	ctx := context.Background()
	f := newImageFixture(t, "")
	rows := twelveRows(date, "样")
	// 白羊座行换成 prompt 上限长度的真实内容（summary 24 字、advice 12 字、avoid 9 字、
	// tip 14 字、必做 14 字），验证满槽不溢出；超限截断 … 行为由纯函数测试覆盖。
	r0 := rows[0]
	r0.OverallScore = 78
	r0.LoveScore, r0.CareerScore, r0.WealthScore, r0.HealthScore = 78, 85, 72, 66
	r0.LoveTip = "多与伴侣进行深度交流"
	r0.CareerTip = "项目推进顺利，注意细节"
	r0.WealthTip = "偏财小有惊喜，见好就收"
	r0.HealthTip = "注意规律作息补充水分"
	r0.Summary = "整体节奏轻快，工作上有贵人点拨，感情宜主动表达"
	r0.MatchConstellation = "狮子座"
	r0.Advice = "上午适合签约谈判与整理"
	r0.Avoid = "避免冲动消费与投资"
	r0.ReunionTip = "旧缘或有回音，宜冷静观察再定"
	r0.SingleTip = "桃花在朋友聚会中出现，多留意"
	r0.JobTip = "面试表现稳，薪资可再谈谈看"
	r0.ReunionScore = 66
	r0.SingleScore = 82
	r0.JobScore = 45
	r0.DailyTodos = model.DailyTodos{
		"给久未联系的朋友发条问候消息",
		"整理桌面和待办清单半小时",
		"睡前拉伸十分钟放松肩颈",
	}
	f.seed(t, rows)

	key, err := f.svc.Ensure(ctx, date, "白羊座")
	if err != nil {
		t.Fatalf("Ensure err = %v, want nil", err)
	}
	rc, err := f.drv.Open(ctx, key)
	if err != nil {
		t.Fatalf("打开盘上 %q err = %v", key, err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("读盘上 %q err = %v", key, err)
	}
	const out = "/tmp/starfuyu-card-sample.jpg"
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatalf("写样图 err = %v", err)
	}
	t.Logf("样图已落盘: %s (%d bytes)", out, len(data))
}
