package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"log"
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

// ImageService 运势图片：gg 绘制（布局照 goLucky service/image.go）+ 内置 OFL 字体 + storage 落盘。
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
}

// NewImageService 构造图片服务。assetsDir 缺省与 config loader 默认一致。
func NewImageService(fortunes FortuneService, driver storage.Driver, cfg *config.Config) ImageService {
	dir := "modules/starfuyu/assets"
	if cfg != nil && cfg.Starfuyu.AssetsDir != "" {
		dir = cfg.Starfuyu.AssetsDir
	}
	return &imageService{fortunes: fortunes, driver: driver, assetsDir: dir}
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

// findFontPath 扫 AssetsDir/fonts 取字典序第一个 ttf/otf/ttc；无命中 → 30502（决定 19）。
func (s *imageService) findFontPath() (string, error) {
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

// render 照 goLucky service/image.go 布局绘制：背景 + 标题/幸运行 + 四段分数与 tips。
func (s *imageService) render(f *model.Fortune) ([]byte, error) {
	fontPath, err := s.findFontPath()
	if err != nil {
		return nil, err
	}

	bgPath := filepath.Join(s.assetsDir, "bgimage", "bg.png")
	bg, err := gg.LoadImage(bgPath)
	if err != nil {
		return nil, errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("加载背景图 %q: %w", bgPath, err))
	}
	imgWidth, imgHeight := bg.Bounds().Dx(), bg.Bounds().Dy()
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.DrawImage(bg, 0, 0)

	const (
		titleFontSize = 72.0
		labelFontSize = 48.0
		textFontSize  = 56.0
		textColor     = "#ffffff"
		shadowColor   = "#000000"
	)
	if err := dc.LoadFontFace(fontPath, titleFontSize); err != nil {
		return nil, fontFail(fontPath, err)
	}

	offsetDown := 700.0
	titleY := 80.0 + offsetDown
	centerX := float64(imgWidth) / 2

	// 标题：星座名称 + 中文日期
	titleText := fmt.Sprintf("%s（%s）", f.Constellation, formatDate(f.Date))
	dc.SetHexColor(shadowColor)
	dc.DrawStringAnchored(titleText, centerX+2, titleY+2, 0.5, 0.5)
	dc.DrawStringAnchored(titleText, centerX+1, titleY+1, 0.5, 0.5)
	dc.SetHexColor(textColor)
	dc.DrawStringAnchored(titleText, centerX, titleY, 0.5, 0.5)

	// 幸运颜色 / 幸运数字
	luckyY := 180.0 + offsetDown
	luckyText := fmt.Sprintf("幸运颜色：%s | 幸运数字：%d", f.LuckyColor.Chinese, f.LuckyNumber)
	dc.SetHexColor(shadowColor)
	dc.DrawStringAnchored(luckyText, centerX+2, luckyY+2, 0.5, 0.5)
	dc.SetHexColor(textColor)
	dc.DrawStringAnchored(luckyText, centerX, luckyY, 0.5, 0.5)

	// 四段运势：整体/爱情/财富/健康（照 goLucky，每段间隔 300px）
	startY := 300.0 + offsetDown
	lineHeight := 300.0
	sections := []struct {
		label string
		tip   string
		score int
	}{
		{"整体运势", f.Summary, f.OverallScore},
		{"爱情运势", f.LoveTip, f.LoveScore},
		{"财富运势", f.WealthTip, f.WealthScore},
		{"健康运势", f.HealthTip, f.HealthScore},
	}
	for i, sec := range sections {
		y := startY + lineHeight*float64(i)
		if err := drawFortuneLine(dc, fontPath, imgWidth, y, sec.label,
			scoreToStars(sec.score), sec.tip, textColor, shadowColor, labelFontSize, textFontSize); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 90}); err != nil {
		return nil, errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("JPEG 编码: %w", err))
	}
	return buf.Bytes(), nil
}

// drawFortuneLine 绘制运势行（标签 + 星级 + 换行 tips），布局照 goLucky。
func drawFortuneLine(dc *gg.Context, fontPath string, width int, y float64,
	label string, stars int, text, textColor, shadowColor string,
	labelFontSize, textFontSize float64) error {
	// 标签
	labelX, labelY := 180.0, y
	if err := dc.LoadFontFace(fontPath, labelFontSize); err != nil {
		return fontFail(fontPath, err)
	}
	dc.SetHexColor(shadowColor)
	dc.DrawStringAnchored(label, labelX+2, labelY+2, 0, 0.5)
	dc.SetHexColor(textColor)
	dc.DrawStringAnchored(label, labelX, labelY, 0, 0.5)

	// 星级（0-100 → 1-5 星）
	starX, starSpacing := 420.0, 40.0
	for i := 0; i < 5; i++ {
		symbol := "☆"
		if i < stars {
			symbol = "★"
		}
		x := starX + float64(i)*starSpacing
		dc.SetHexColor(shadowColor)
		dc.DrawStringAnchored(symbol, x+1, labelY+1, 0.5, 0.5)
		dc.SetHexColor(textColor)
		dc.DrawStringAnchored(symbol, x, labelY, 0.5, 0.5)
	}

	// 说明文字：星星下方换行显示
	textX := 180.0
	maxWidth := float64(width) - textX - 50
	if err := dc.LoadFontFace(fontPath, textFontSize); err != nil {
		return fontFail(fontPath, err)
	}
	text = wrapText(text, maxWidth, dc)
	textY := labelY + 60.0
	dc.SetHexColor(shadowColor)
	dc.DrawStringWrapped(text, textX+3, textY+3, 0, 0, maxWidth, 1.8, gg.AlignLeft)
	dc.DrawStringWrapped(text, textX+2, textY+2, 0, 0, maxWidth, 1.8, gg.AlignLeft)
	dc.DrawStringWrapped(text, textX+1, textY+1, 0, 0, maxWidth, 1.8, gg.AlignLeft)
	dc.SetHexColor(textColor)
	dc.DrawStringWrapped(text, textX, textY, 0, 0, maxWidth, 1.8, gg.AlignLeft)
	return nil
}

// scoreToStars 分数（0-100）→ 星级（1-5），照 goLucky。
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

// wrapText 每 20 字强制换行（照 goLucky），随后交给 DrawStringWrapped 按宽度收尾。
func wrapText(text string, maxWidth float64, dc *gg.Context) string {
	if len(text) == 0 {
		return text
	}
	var lines []string
	var current []rune
	for _, r := range text {
		if r == '\n' {
			if len(current) > 0 {
				lines = append(lines, string(current))
				current = nil
			}
			continue
		}
		current = append(current, r)
		if len(current) >= 20 {
			lines = append(lines, string(current))
			current = nil
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return strings.Join(lines, "\n")
}

// formatDate YYYY-MM-DD → YYYY年MM月DD日（照 goLucky）。
func formatDate(dateStr string) string {
	parts := strings.Split(dateStr, "-")
	if len(parts) == 3 {
		return fmt.Sprintf("%s年%s月%s日", parts[0], parts[1], parts[2])
	}
	return dateStr
}
