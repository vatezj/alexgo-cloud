package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fogleman/gg"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
)

// ImageService 运势图片：gg 按分享卡模板布局绘制（share_card.go 常量表）+ 内置 OFL 字体 + storage 落盘。
// 决定 19：字体缺失/加载失败 → 30502，不做静默 ASCII 回退。
type ImageService interface {
	// Ensure 生成（或已存在则跳过）当日星座图，返回 storage rel key：fortune/v2/{date}/{constellation}.jpg
	Ensure(ctx context.Context, date, constellation string) (key string, err error)
	// GenerateAll 遍历 12 星座 Ensure；单张失败记日志继续，返回成功数。
	// 全部成功 err=nil；有失败 err 为 30502 聚合错误（首错包在链上）。
	GenerateAll(ctx context.Context, date string) (ok int, err error)
}

type imageService struct {
	fortunes  FortuneService
	driver    storage.Driver
	assetsDir string
	fontFile  string // starfuyu.font_file：字体路径（源码树外）；空回退 assets/fonts glob
	bgFile    string // starfuyu.bg_file：底图路径（源码树外）；空回退 assets/bgimage/bg.png
	cuteDir   string // starfuyu.cute_dir：角色抠图目录（源码树外）；空串渲染时 30502 不回退
}

// NewImageService 构造图片服务。assetsDir 缺省与 config loader 默认一致；fontFile/bgFile/cuteDir 直取 cfg。
func NewImageService(fortunes FortuneService, driver storage.Driver, cfg *config.Config) ImageService {
	dir := "modules/starfuyu/assets"
	var fontFile, bgFile, cuteDir string
	if cfg != nil {
		if cfg.Starfuyu.AssetsDir != "" {
			dir = cfg.Starfuyu.AssetsDir
		}
		fontFile = cfg.Starfuyu.FontFile
		bgFile = cfg.Starfuyu.BgFile
		cuteDir = cfg.Starfuyu.CuteDir
	}
	return &imageService{fortunes: fortunes, driver: driver, assetsDir: dir, fontFile: fontFile, bgFile: bgFile, cuteDir: cuteDir}
}

// resolveCutePath 角色抠图定位：cute_dir 空 → 30502「未配置」；slug 未命中 → 30502
// 「非法星座」；文件缺失 → 30502 指明完整路径。不静默跳过——与 font_file/bg_file 同精神。
func (s *imageService) resolveCutePath(constellation string) (string, error) {
	if s.cuteDir == "" {
		return "", errs.New(errs.CodeStarfuyuImageFailed,
			fmt.Errorf("cute_dir 未配置（starfuyu.cute_dir），无法定位角色素材"))
	}
	slug, ok := constellationSlug(constellation)
	if !ok {
		return "", errs.New(errs.CodeStarfuyuImageFailed,
			fmt.Errorf("非法星座无法映射 cute slug: %q", constellation))
	}
	path := filepath.Join(s.cuteDir, "cute_"+slug+"_cut.png")
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return "", errs.New(errs.CodeStarfuyuImageFailed,
			fmt.Errorf("角色抠图不存在: %s", path))
	}
	return path, nil
}

// resolveBgPath 底图定位：bg_file 配置非空 → 直用（文件缺失 30502 指明 bg_file，
// 不静默回退——配置错了必须炸，与 font_file 同精神）；空串 → 回退 AssetsDir/bgimage/bg.png。
func (s *imageService) resolveBgPath() (string, error) {
	if s.bgFile != "" {
		fi, err := os.Stat(s.bgFile)
		if err != nil || fi.IsDir() {
			return "", errs.New(errs.CodeStarfuyuImageFailed,
				fmt.Errorf("bg_file 配置指向的底图不存在: %s", s.bgFile))
		}
		return s.bgFile, nil
	}
	return filepath.Join(s.assetsDir, "bgimage", "bg.png"), nil
}

// fortuneImageKey 校验入参后生成 rel key：fortune/v2/{date}/{constellation}.jpg。
// v1=旧布局，v2=分享卡模板——换模板升版 key，防旧图被 Ensure 幂等跳过。
// storage 合同要求 rel 由 service 生成、
// 不得混入未校验用户输入——date 钉死 YYYY-MM-DD、星座钉死 12 枚举，杜绝路径穿越。
func fortuneImageKey(date, constellation string) (string, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", errs.New(errs.CodeParamInvalid, fmt.Errorf("date 须为 YYYY-MM-DD: %q", date))
	}
	if !IsValidConstellation(constellation) {
		return "", errs.New(errs.CodeParamInvalid, fmt.Errorf("非法星座: %q", constellation))
	}
	return fmt.Sprintf("fortune/v2/%s/%s.jpg", date, constellation), nil
}

func (s *imageService) Ensure(ctx context.Context, date, constellation string) (string, error) {
	key, err := fortuneImageKey(date, constellation)
	if err != nil {
		return "", err
	}

	// 已存在则跳过（幂等）：Open 命中直接返回。
	if rc, err := s.driver.Open(ctx, key); err == nil {
		_ = rc.Close()
		return key, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("探测已存图片 %q: %w", key, err)
	}

	row, err := s.fortunes.GetOne(ctx, date, constellation)
	if err != nil {
		return "", err // 30501 原样透传
	}

	buf, err := s.render(row)
	if err != nil {
		return "", err
	}

	if err := s.driver.Save(ctx, key, bytes.NewReader(buf)); err != nil {
		return "", errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("图片落盘失败 %q: %w", key, err))
	}
	return key, nil
}

func (s *imageService) GenerateAll(ctx context.Context, date string) (int, error) {
	ok, failed := 0, 0
	var firstErr error
	for _, c := range ConstellationList {
		if _, err := s.Ensure(ctx, date, c); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("[Image] 运势图生成失败 date=%s constellation=%s err=%v", date, c, err)
			continue // 单张失败继续下一张
		}
		ok++
	}
	if failed == 0 {
		return ok, nil
	}
	return ok, errs.New(errs.CodeStarfuyuImageFailed,
		fmt.Errorf("运势图 %d/%d 张失败，首错: %w", failed, len(ConstellationList), firstErr))
}

// findFontPath 字体定位：font_file 配置非空 → 直用（文件缺失 30502 指明 font_file，
// 不静默回退——配置错了必须炸）；空串 → 回退扫 AssetsDir/fonts 取字典序首个 ttf/otf/ttc。
// 两路无命中 → 30502（决定 19，不回退 ASCII）。字体放运行时目录（data/fonts），不放源码树。
func (s *imageService) findFontPath() (string, error) {
	if s.fontFile != "" {
		fi, err := os.Stat(s.fontFile)
		if err != nil || fi.IsDir() {
			return "", errs.New(errs.CodeStarfuyuImageFailed,
				fmt.Errorf("font_file 配置指向的字体不存在: %s", s.fontFile))
		}
		return s.fontFile, nil
	}
	matches, err := filepath.Glob(filepath.Join(s.assetsDir, "fonts", "*"))
	if err != nil {
		return "", errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("扫描字体目录失败: %w", err))
	}
	cands := make([]string, 0, len(matches))
	for _, m := range matches {
		switch strings.ToLower(filepath.Ext(m)) {
		case ".ttf", ".otf", ".ttc":
			if fi, statErr := os.Stat(m); statErr == nil && !fi.IsDir() {
				cands = append(cands, m)
			}
		}
	}
	if len(cands) == 0 {
		return "", errs.New(errs.CodeStarfuyuImageFailed,
			fmt.Errorf("字体缺失：%s/fonts 无 ttf/otf/ttc 命中（决定 19，不回退 ASCII）", s.assetsDir))
	}
	sort.Strings(cands)
	return cands[0], nil
}

// fontFail 统一 30502：字体文件加载失败。
func fontFail(path string, err error) error {
	return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("加载字体 %q: %w", path, err))
}

// render 分享卡模板（bg.png 1728×2397）逐槽绘制：上卡五槽 + 四运程条 + 三小卡文案
// + 三条进度条 + 今日必做三行。模板没印的不画（无标题/幸运色/星级）；
// 空字段留白不画 -/—；进度条先铺底再画填充段；输出 JPEG q90。
// 槽位坐标与字号集中在 shareCardLayout/shareFont* 常量表（share_card.go）。
func (s *imageService) render(f *model.Fortune) ([]byte, error) {
	fontPath, err := s.findFontPath()
	if err != nil {
		return nil, err
	}

	bgPath, err := s.resolveBgPath()
	if err != nil {
		return nil, err
	}
	bg, err := gg.LoadImage(bgPath)
	if err != nil {
		return nil, errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("加载背景图 %q: %w", bgPath, err))
	}

	// 角色抠图定位 fail-fast（先于任何绘制：缺素材整张不落盘）。
	cutePath, err := s.resolveCutePath(f.Constellation)
	if err != nil {
		return nil, err
	}
	mascot, err := gg.LoadImage(cutePath)
	if err != nil {
		return nil, errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("加载角色抠图 %q: %w", cutePath, err))
	}

	dc := gg.NewContext(bg.Bounds().Dx(), bg.Bounds().Dy())
	dc.DrawImage(bg, 0, 0)

	// 顶部星座名：珊瑚粉大字 + 白描边（f.Constellation 必非空——key 校验已钉 12 枚举）。
	if err := drawShareTitle(dc, fontPath, f.Constellation, shareCardLayout.Title); err != nil {
		return nil, err
	}

	// 左上角角色：contain 等比缩放居中于 Mascot 槽（各张 bbox 比例不同，不越界不变形）。
	drawShareMascot(dc, mascot, shareCardLayout.Mascot)

	// 上卡五槽：标签已印底图，只画冒号后的值。
	upper := []struct {
		text string
		r    slot
		n    int // 最大行数
	}{
		{f.Summary, shareCardLayout.Summary, 2},
		{fmt.Sprintf("%d分", clampScore(f.OverallScore)), shareCardLayout.Score, 1},
		{f.MatchConstellation, shareCardLayout.Match, 1},
		{f.Advice, shareCardLayout.Advice, 1},
		{f.Avoid, shareCardLayout.Avoid, 1},
	}
	for _, u := range upper {
		if err := drawShareText(dc, fontPath, shareFontUpper, u.text, u.r, u.n, false); err != nil {
			return nil, err
		}
	}

	// 四条运程：「N分 · tip」单行，胶囊内垂直居中。
	lanes := []struct {
		score int
		tip   string
		r     slot
	}{
		{f.WealthScore, f.WealthTip, shareCardLayout.LaneWealth},
		{f.CareerScore, f.CareerTip, shareCardLayout.LaneCareer},
		{f.LoveScore, f.LoveTip, shareCardLayout.LaneLove},
		{f.HealthScore, f.HealthTip, shareCardLayout.LaneHealth},
	}
	for _, l := range lanes {
		text := fmt.Sprintf("%d分", clampScore(l.score))
		if l.tip != "" {
			text += " · " + l.tip
		}
		if err := drawShareText(dc, fontPath, shareFontLane, text, l.r, 1, true); err != nil {
			return nil, err
		}
	}

	// 三小卡文案（tip，最多 2 行）。
	cards := []struct {
		tip string
		r   slot
	}{
		{f.ReunionTip, shareCardLayout.Reunion},
		{f.SingleTip, shareCardLayout.Single},
		{f.JobTip, shareCardLayout.Job},
	}
	for _, c := range cards {
		if err := drawShareText(dc, fontPath, shareFontCard, c.tip, c.r, 2, false); err != nil {
			return nil, err
		}
	}

	// 三小卡进度条：先铺底（浅色同色矩形）再画深色圆角填充段；心形端点在槽外。
	bars := []struct {
		score int
		fill  string
		r     struct{ x, y, w, h float64 }
	}{
		{f.ReunionScore, shareBarReunion, shareCardLayout.BarReunion},
		{f.SingleScore, shareBarSingle, shareCardLayout.BarSingle},
		{f.JobScore, shareBarJob, shareCardLayout.BarJob},
	}
	for _, b := range bars {
		dc.SetHexColor(shareTrackColor)
		dc.DrawRectangle(b.r.x, b.r.y, b.r.w, b.r.h)
		dc.Fill()
		if fw := progressFillWidth(b.r.w, b.score); fw > 0 {
			r := b.r.h / 2
			if fw/2 < r {
				r = fw / 2
			}
			dc.SetHexColor(b.fill)
			dc.DrawRoundedRectangle(b.r.x, b.r.y, fw, b.r.h, r)
			dc.Fill()
		}
	}

	// 今日必做：最多 3 条逐行画；不足留空行（底图勾选框保留）。
	for i, todo := range f.DailyTodos {
		if i >= len(shareCardLayout.Todos) {
			break
		}
		if err := drawShareText(dc, fontPath, shareFontTodo, todo, shareCardLayout.Todos[i], 1, true); err != nil {
			return nil, err
		}
	}

	// 底部粉条口号：白字、槽内水平居中；空值留白（粉条原样）。
	if err := drawShareSlogan(dc, fontPath, f.Slogan, shareCardLayout.Slogan); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 90}); err != nil {
		return nil, errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("JPEG 编码: %w", err))
	}
	return buf.Bytes(), nil
}

// drawShareText 槽内绘制：wrapByWidth 按槽宽断行 → truncateLines 截断（末行补 …）→
// 逐行 DrawString。vCenter 且单行时槽内垂直居中（运程条/必做）；空文本留白。
// 截断末行含 … 时逐字回退到槽宽内，保证不溢出 rect。
func drawShareText(dc *gg.Context, fontPath string, size float64, text string, r slot, maxLines int, vCenter bool) error {
	if text == "" {
		return nil
	}
	if err := dc.LoadFontFace(fontPath, size); err != nil {
		return fontFail(fontPath, err)
	}
	measure := func(s string) float64 {
		w, _ := dc.MeasureString(s)
		return w
	}
	lines := truncateLines(wrapByWidth(text, r.w, measure), maxLines)
	if len(lines) == 0 {
		return nil
	}
	// 末行带 … 的截断串可能略超宽（truncateLines 只拼 … 不测宽），回退到槽宽内。
	last := lines[len(lines)-1]
	for measure(last) > r.w && len([]rune(last)) > 1 {
		last = string([]rune(last)[:len([]rune(last))-2]) + "…"
	}
	lines[len(lines)-1] = last

	dc.SetHexColor(shareTextColor)
	if vCenter && len(lines) == 1 {
		dc.DrawString(lines[0], r.x, r.y+r.h/2+size*0.36)
		return nil
	}
	// 顶对齐：首行基线 = 槽顶 + 上伸距（NotoSansSC 实测 ink top→baseline ≈ 0.84em）。
	lineH := size * 1.2
	for i, line := range lines {
		if line == "" {
			continue
		}
		dc.DrawString(line, r.x, r.y+size*0.84+lineH*float64(i))
	}
	return nil
}

// drawShareTitle 顶部星座名：槽内水平居中 + 垂直近似居中，
// 白描边（16 向圆周偏移 ±shareTitleStroke 画底）+ 珊瑚粉面字。
// 独立于 drawShareText（描边+居中两个差异点，不值得给主函数加参）。
func drawShareTitle(dc *gg.Context, fontPath, text string, r slot) error {
	if text == "" {
		return nil
	}
	if err := dc.LoadFontFace(fontPath, shareFontTitle); err != nil {
		return fontFail(fontPath, err)
	}
	w, _ := dc.MeasureString(text)
	cx := r.x + (r.w-w)/2
	cy := r.y + r.h/2 + shareFontTitle*0.36 // 与 drawShareText 基线惯例一致
	for i := 0; i < 16; i++ {
		a := 2 * math.Pi * float64(i) / 16
		dc.SetHexColor("#FFFFFF")
		dc.DrawString(text, cx+shareTitleStroke*math.Cos(a), cy+shareTitleStroke*math.Sin(a))
	}
	dc.SetHexColor(shareTitleColor)
	dc.DrawString(text, cx, cy)
	return nil
}

// drawShareMascot 角色抠图 contain 缩放绘制：s=min(槽宽/图宽, 槽高/图高)，
// 居中于槽。gg DrawImageAnchored 走当前矩阵（BiLinear 重采样），Scale 后必须 Identity 复位。
func drawShareMascot(dc *gg.Context, mascot image.Image, r slot) {
	b := mascot.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	if w <= 0 || h <= 0 {
		return
	}
	s := r.w / w
	if rs := r.h / h; rs < s {
		s = rs
	}
	px := r.x + (r.w-w*s)/2
	py := r.y + (r.h-h*s)/2
	dc.Translate(px, py)
	dc.Scale(s, s)
	dc.DrawImageAnchored(mascot, 0, 0, 0, 0)
	dc.Identity()
}

// drawShareSlogan 底部粉条口号：白字、槽内水平+垂直居中；wrap 按槽宽、单行截断 …。
// 独立于 drawShareText（颜色白、水平居中两个差异点，不值得给主函数加参）。
func drawShareSlogan(dc *gg.Context, fontPath, text string, r slot) error {
	if text == "" {
		return nil
	}
	if err := dc.LoadFontFace(fontPath, shareFontSlogan); err != nil {
		return fontFail(fontPath, err)
	}
	measure := func(s string) float64 {
		w, _ := dc.MeasureString(s)
		return w
	}
	lines := truncateLines(wrapByWidth(text, r.w, measure), 1)
	if len(lines) == 0 {
		return nil
	}
	line := lines[0]
	for measure(line) > r.w && len([]rune(line)) > 1 {
		line = string([]rune(line)[:len([]rune(line))-2]) + "…"
	}
	w, _ := dc.MeasureString(line)
	dc.SetHexColor("#FFFFFF")
	dc.DrawString(line, r.x+(r.w-w)/2, r.y+r.h/2+shareFontSlogan*0.36)
	return nil
}

// scoreToStars 分数（0-100）→ 星级（1-5），article.go 公众号文案引用，保留。
func scoreToStars(score int) int {
	switch {
	case score >= 80:
		return 5
	case score >= 60:
		return 4
	case score >= 40:
		return 3
	case score >= 20:
		return 2
	}
	return 1
}

// formatDate YYYY-MM-DD → YYYY年MM月DD日（article.go 引用，保留）。
func formatDate(dateStr string) string {
	parts := strings.Split(dateStr, "-")
	if len(parts) == 3 {
		return fmt.Sprintf("%s年%s月%s日", parts[0], parts[1], parts[2])
	}
	return dateStr
}
