package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
)

// ArticleService 公众号运势文章：HTML 生成（照 goLucky handler/fortune.go）+ 草稿箱上传。
type ArticleService interface {
	// BuildHTML 生成整版（12 星座）或单星座 HTML；constellation=="" 整版。
	BuildHTML(ctx context.Context, date, constellation string) (string, error)
	// UploadToMP 封面素材 + 整版 HTML → 草稿箱；默认 1 整版篇（controller 裁定）。
	// 密钥空配 → 立即 30507 不打网络；微信 API 失败 → 30507。
	UploadToMP(ctx context.Context, date string) ([]UploadedArticle, error)
}

// UploadedArticle 草稿箱上传结果（spec §4.2 只要求 title/media_id 列表）。
type UploadedArticle struct {
	Title   string `json:"title"`
	MediaID string `json:"media_id"`
}

type articleService struct {
	fortunes FortuneService
	images   ImageService
	mp       MPClient
	driver   storage.Driver
	cfg      *config.Config
}

// NewArticleService 组装文章服务。
// driver 供 UploadToMP 经 storage.Driver.Open(key) 读封面字节（preflight 裁定 7→8）。
func NewArticleService(fortunes FortuneService, images ImageService, mp MPClient,
	driver storage.Driver, cfg *config.Config) ArticleService {
	return &articleService{fortunes: fortunes, images: images, mp: mp, driver: driver, cfg: cfg}
}

// mpConfigured 密钥非空（验收 §11.4 前置）。
func (s *articleService) mpConfigured() bool {
	if s.cfg == nil {
		return false
	}
	return s.cfg.Starfuyu.WechatMp.AppID != "" && s.cfg.Starfuyu.WechatMp.AppSecret != ""
}

// BuildHTML 照 goLucky _generateFortuneArticleHTML / _generateSingle…（去 gin，date 以入参为准）。
func (s *articleService) BuildHTML(ctx context.Context, date, constellation string) (string, error) {
	if constellation != "" {
		if !IsValidConstellation(constellation) {
			return "", errs.New(errs.CodeParamInvalid, fmt.Errorf("非法星座: %q", constellation))
		}
		row, err := s.fortunes.GetOne(ctx, date, constellation)
		if err != nil {
			return "", err // 30501 原样透传
		}
		return generateSingleConstellationArticleHTML(row, date), nil
	}

	rows, err := s.fortunes.GetByDate(ctx, date)
	if err != nil {
		return "", err
	}
	return generateFortuneArticleHTML(rows, date), nil
}

// UploadToMP 默认整版 1 篇：Ensure 封面 → Open 字节 → AddDraft。
func (s *articleService) UploadToMP(ctx context.Context, date string) ([]UploadedArticle, error) {
	// 密钥空配：立即 30507，不打网络（短路在素材/HTML 流水线之前）。
	if !s.mpConfigured() {
		return nil, errs.New(errs.CodeStarfuyuMpUploadFailed,
			fmt.Errorf("wechat_mp appid/secret 未配置"))
	}

	// 1. 封面：Ensure 白羊座图（整版数据应含 12 星座）→ storage key。
	thumbKey, err := s.images.Ensure(ctx, date, ConstellationList[0])
	if err != nil {
		return nil, err // 30501/30502 原样透传
	}

	// 2. 读封面字节（preflight 裁定：ArticleService 经 Driver.Open）。
	data, err := readStorageAll(ctx, s.driver, thumbKey)
	if err != nil {
		return nil, errs.New(errs.CodeStarfuyuMpUploadFailed,
			fmt.Errorf("读封面素材 %q: %w", thumbKey, err))
	}

	// 3. 上传永久图片素材 → thumb_media_id。
	thumbMediaID, err := s.mp.UploadImageMaterial(ctx, data)
	if err != nil {
		return nil, wrapMpErr(err)
	}

	// 4. 整版 HTML + AddDraft（默认 1 篇）。
	html, err := s.BuildHTML(ctx, date, "")
	if err != nil {
		return nil, err
	}
	dateStr := formatDate(date)
	weekdayStr := weekdayChinese(mustParseDate(date).Weekday())
	title := fmt.Sprintf("十二星座每日好运｜%s", dateStr)
	digest := fmt.Sprintf("今天是%s %s，一起来看看十二星座的今日运势吧！", dateStr, weekdayStr)

	mediaID, err := s.mp.AddDraft(ctx, []MPArticle{{
		Title:        title,
		Content:      html,
		Digest:       digest,
		Author:       "星福柚",
		ShowCoverPic: 1,
		ThumbMediaID: thumbMediaID,
	}})
	if err != nil {
		return nil, wrapMpErr(err)
	}

	return []UploadedArticle{{Title: title, MediaID: mediaID}}, nil
}

// wrapMpErr MPClient 合同外的裸错误兜底包 30507；已是 30507 则透传。
func wrapMpErr(err error) error {
	if err == nil || errs.Is(err, errs.CodeStarfuyuMpUploadFailed) {
		return err
	}
	return errs.New(errs.CodeStarfuyuMpUploadFailed, err)
}

// readStorageAll 读 storage key 全量字节并关闭。
func readStorageAll(ctx context.Context, d storage.Driver, key string) ([]byte, error) {
	rc, err := d.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// mustParseDate YYYY-MM-DD → time.Time；非法返回零值（dateStr 已由 formatDate 兜底）。
func mustParseDate(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}
	}
	return t
}

// weekdayChinese 中文星期（照 goLucky handler）。
func weekdayChinese(w time.Weekday) string {
	m := map[time.Weekday]string{
		time.Sunday:    "星期日",
		time.Monday:    "星期一",
		time.Tuesday:   "星期二",
		time.Wednesday: "星期三",
		time.Thursday:  "星期四",
		time.Friday:    "星期五",
		time.Saturday:  "星期六",
	}
	return m[w]
}

// constellationMonth 星座日期范围（照 goLucky handler）。
func constellationMonth(constellation string) string {
	m := map[string]string{
		"白羊座": "（3月21日-4月19日）",
		"金牛座": "（4月20日-5月20日）",
		"双子座": "（5月21日-6月21日）",
		"巨蟹座": "（6月22日-7月22日）",
		"狮子座": "（7月23日-8月22日）",
		"处女座": "（8月23日-9月22日）",
		"天秤座": "（9月23日-10月23日）",
		"天蝎座": "（10月24日-11月22日）",
		"射手座": "（11月23日-12月21日）",
		"摩羯座": "（12月22日-1月19日）",
		"水瓶座": "（1月20日-2月18日）",
		"双鱼座": "（2月19日-3月20日）",
	}
	return m[constellation]
}

// starsToString 分级 → 「N分」（照 goLucky handler）。
func starsToString(stars int) string {
	switch stars {
	case 1, 2, 3, 4, 5:
		return fmt.Sprintf("%d分", stars)
	}
	return "🌟"
}

// luckyTextColor 亮色背景切黑字（照 goLucky handler 亮度启发式）。
func luckyTextColor(colorHex string) string {
	textColor := "#fff"
	if len(colorHex) == 7 && colorHex[0] == '#' {
		var r, g, b int
		if n, err := fmt.Sscanf(colorHex, "#%02x%02x%02x", &r, &g, &b); n == 3 && err == nil {
			brightness := (r*299 + g*587 + b*114) / 1000
			if brightness > 125 {
				textColor = "#000"
			}
		}
	}
	return textColor
}

// writeLuckyBox 幸运颜色/数字色块（整版与单篇共用）。
func writeLuckyBox(b *strings.Builder, f *model.Fortune) {
	bg := f.LuckyColor.Hex
	text := luckyTextColor(bg)
	fmt.Fprintf(b, `<div style="background-color: %s; color: %s; padding: 15px; border-radius: 8px; margin: 15px 0;">`, bg, text)
	fmt.Fprintf(b, `<p style="margin: 5px 0; font-size: 15px;"><strong>幸运颜色：</strong>%s</p>`, f.LuckyColor.Chinese)
	fmt.Fprintf(b, `<p style="margin: 5px 0; font-size: 15px;"><strong>幸运数字：</strong>%d</p>`, f.LuckyNumber)
	b.WriteString(`</div>`)
}

// writeScoreSection 五维分数段（整体/爱情/事业/财富/健康）。
func writeScoreSection(b *strings.Builder, f *model.Fortune) {
	fmt.Fprintf(b, `<p style="margin: 15px 0; line-height: 1.8;"><strong style="color: #2c3e50; font-size: 16px;">整体运势（%s）</strong><br><span style="color: #555; font-size: 15px;">%s</span></p>`,
		starsToString(scoreToStars(f.OverallScore)), f.Summary)
	fmt.Fprintf(b, `<p style="margin: 15px 0; line-height: 1.8;"><strong style="color: #e91e63; font-size: 16px;">爱情运势（%s）</strong><br><span style="color: #555; font-size: 15px;">%s</span></p>`,
		starsToString(scoreToStars(f.LoveScore)), f.LoveTip)
	fmt.Fprintf(b, `<p style="margin: 15px 0; line-height: 1.8;"><strong style="color: #2196f3; font-size: 16px;">事业运势（%s）</strong><br><span style="color: #555; font-size: 15px;">%s</span></p>`,
		starsToString(scoreToStars(f.CareerScore)), f.CareerTip)
	fmt.Fprintf(b, `<p style="margin: 15px 0; line-height: 1.8;"><strong style="color: #ff9800; font-size: 16px;">财富运势（%s）</strong><br><span style="color: #555; font-size: 15px;">%s</span></p>`,
		starsToString(scoreToStars(f.WealthScore)), f.WealthTip)
	fmt.Fprintf(b, `<p style="margin: 15px 0; line-height: 1.8;"><strong style="color: #4caf50; font-size: 16px;">健康运势（%s）</strong><br><span style="color: #555; font-size: 15px;">%s</span></p>`,
		starsToString(scoreToStars(f.HealthScore)), f.HealthTip)
}

// generateSingleConstellationArticleHTML 单星座文章（照 goLucky _generateSingle…，date 以入参为准）。
func generateSingleConstellationArticleHTML(f *model.Fortune, date string) string {
	t := mustParseDate(date)
	dateStr := formatDate(date)
	weekdayStr := weekdayChinese(t.Weekday())

	var b strings.Builder
	b.WriteString(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; line-height: 1.8; color: #333; max-width: 680px; margin: 0 auto; padding: 20px;">`)
	fmt.Fprintf(&b, `<p style="font-size: 16px; text-indent: 2em; margin: 15px 0; color: #555;">今天是%s %s，来看看你的专属好运吧！✨ 愿你今天充满活力，把握每一个机会！</p>`, dateStr, weekdayStr)
	b.WriteString(`<hr style="border: none; border-top: 1px solid #e0e0e0; margin: 25px 0;">`)
	fmt.Fprintf(&b, `<h2 style="font-size: 20px; font-weight: bold; color: #e74c3c; margin: 30px 0 15px 0; padding: 10px 0; border-left: 4px solid #e74c3c; padding-left: 15px;">%s %s</h2>`,
		f.Constellation, constellationMonth(f.Constellation))
	writeLuckyBox(&b, f)
	writeScoreSection(&b, f)
	b.WriteString(`<hr style="border: none; border-top: 2px solid #3498db; margin: 30px 0;">`)
	b.WriteString(`<p style="text-align: center; color: #666; font-size: 14px; margin: 10px 0;"><em>以上运势仅供参考，祝您拥有美好的一天！</em></p>`)
	b.WriteString(`<p style="text-align: center; color: #999; font-size: 12px; margin: 15px 0;">每天更新，记得关注我们获取最新运势~</p>`)
	b.WriteString(`</div>`)
	return b.String()
}

// generateFortuneArticleHTML 整版 12 星座文章（照 goLucky _generateFortuneArticleHTML）。
func generateFortuneArticleHTML(fortunes []*model.Fortune, date string) string {
	t := mustParseDate(date)
	dateStr := formatDate(date)
	weekdayStr := weekdayChinese(t.Weekday())

	var b strings.Builder
	b.WriteString(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; line-height: 1.8; color: #333; max-width: 680px; margin: 0 auto; padding: 20px;">`)
	fmt.Fprintf(&b, `<p style="font-size: 16px; text-indent: 2em; margin: 15px 0; color: #555;">今天是%s %s，新的一天开始了！想知道你的星座今天会有什么惊喜吗？让我们一起来看看十二星座的今日运势，把握每一个机会，让好运常伴左右！</p>`, dateStr, weekdayStr)
	b.WriteString(`<hr style="border: none; border-top: 1px solid #e0e0e0; margin: 25px 0;">`)

	fortuneMap := make(map[string]*model.Fortune, len(fortunes))
	for _, f := range fortunes {
		if f != nil {
			fortuneMap[f.Constellation] = f
		}
	}

	for idx, constellation := range ConstellationList {
		f, ok := fortuneMap[constellation]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, `<h2 style="font-size: 20px; font-weight: bold; color: #e74c3c; margin: 30px 0 15px 0; padding: 10px 0; border-left: 4px solid #e74c3c; padding-left: 15px;">%s %s</h2>`,
			f.Constellation, constellationMonth(f.Constellation))
		writeLuckyBox(&b, f)
		writeScoreSection(&b, f)
		// 分隔条照 goLucky：非末位星座之间插入（跳过缺失行不补条）。
		if idx < len(ConstellationList)-1 && hasFollowing(fortuneMap, idx) {
			b.WriteString(`<div style="text-align: center; margin: 25px 0; padding: 15px 0; border-top: 1px dashed #ddd; border-bottom: 1px dashed #ddd;">`)
			b.WriteString(`<p style="color: #999; font-size: 14px; margin: 0;">✨ 继续查看下一个星座的运势 ✨</p>`)
			b.WriteString(`</div>`)
		}
	}

	b.WriteString(`<p style="text-align: center; color: #999; font-size: 13px; margin: 30px 0 15px 0; font-style: italic;">━━━━━━━━━━━━━━━━━━━━</p>`)
	b.WriteString(`<p style="text-align: center; color: #666; font-size: 14px; margin: 10px 0;"><em>以上运势仅供参考，祝您拥有美好的一天！</em></p>`)
	b.WriteString(`<p style="text-align: center; color: #999; font-size: 12px; margin: 15px 0;">每天更新，记得关注我们获取最新运势~</p>`)
	b.WriteString(`</div>`)
	return b.String()
}

// hasFollowing idx 之后是否还有已存在的星座行（决定是否加分隔条）。
func hasFollowing(m map[string]*model.Fortune, idx int) bool {
	for i := idx + 1; i < len(ConstellationList); i++ {
		if _, ok := m[ConstellationList[i]]; ok {
			return true
		}
	}
	return false
}
