package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
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
	daily    repository.DailyRepository
}

// NewArticleService 组装文章服务。
// driver 供 UploadToMP 经 storage.Driver.Open(key) 读封面字节（preflight 裁定 7→8）。
func NewArticleService(fortunes FortuneService, images ImageService, mp MPClient,
	driver storage.Driver, cfg *config.Config, daily repository.DailyRepository) ArticleService {
	return &articleService{fortunes: fortunes, images: images, mp: mp, driver: driver, cfg: cfg, daily: daily}
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
	// 日级星象：miss → nil，buildImageArticleHTML 内降级少画两块。
	var daily *model.DailyHoroscope
	if s.daily != nil {
		// daily 行恒 tid=0：读取钉 contentCtx，防请求 ctx 带租户时 gormplugin 误过滤（终审 Important-1）。
		row, _, dErr := s.daily.GetByDate(contentCtx(ctx), date)
		if dErr != nil {
			return "", dErr
		}
		daily = row
	}
	return buildImageArticleHTML(daily, rows, date, func(c string) string {
		key, kErr := fortuneImageKey(date, c)
		if kErr != nil {
			return "" // key 校验过（rows 来自库）；理论不可达
		}
		return "/api/public/starfuyu/image/" + key
	}), nil
}

// UploadToMP 默认整版 1 篇（图片版，spec §5）：GenerateAll 12 张 v3 海报 →
// 封面永久素材 → 12 张正文图逐传 uploadimg（任一失败即中止，不发半成品草稿）→
// buildImageArticleHTML（CDN URL）→ AddDraft。
func (s *articleService) UploadToMP(ctx context.Context, date string) ([]UploadedArticle, error) {
	// 密钥空配：立即 30507，不打网络（短路在素材/HTML 流水线之前）。
	if !s.mpConfigured() {
		return nil, errs.New(errs.CodeStarfuyuMpUploadFailed,
			fmt.Errorf("wechat_mp appid/secret 未配置"))
	}

	// 1. 12 张 v3 海报齐（单张失败 err 透传——30501/30502）。
	if _, err := s.images.GenerateAll(ctx, date); err != nil {
		return nil, err
	}

	// 2. 封面：cover_dir/{date}.png（scripts/starfuyu_cover.py 合成图）优先；
	// 缺失/未配置 → 回退白羊海报（软降级——脚本没跑不阻断上传）。
	// date 先过格式校验再拼路径（防 ../ 穿越；非法 date 后续 fortuneImageKey 也会 10001）。
	var thumbData []byte
	if _, perr := time.Parse("2006-01-02", date); perr == nil && s.cfg != nil && s.cfg.Starfuyu.CoverDir != "" {
		if b, rerr := os.ReadFile(filepath.Join(s.cfg.Starfuyu.CoverDir, date+".png")); rerr == nil && len(b) > 0 {
			thumbData = b
		}
	}
	if thumbData == nil {
		thumbKey, err := s.images.Ensure(ctx, date, ConstellationList[0])
		if err != nil {
			return nil, err
		}
		thumbData, err = readStorageAll(ctx, s.driver, thumbKey)
		if err != nil {
			return nil, errs.New(errs.CodeStarfuyuMpUploadFailed,
				fmt.Errorf("读封面素材 %q: %w", thumbKey, err))
		}
	}
	thumbMediaID, err := s.mp.UploadImageMaterial(ctx, thumbData)
	if err != nil {
		return nil, wrapMpErr(err)
	}

	// 3. 正文图逐传 uploadimg → CDN URL；任一失败直接返回（AddDraft 未触达）。
	imgURLs := make(map[string]string, len(ConstellationList))
	for _, c := range ConstellationList {
		key, err := fortuneImageKey(date, c)
		if err != nil {
			return nil, err
		}
		imgBytes, err := readStorageAll(ctx, s.driver, key)
		if err != nil {
			return nil, errs.New(errs.CodeStarfuyuMpUploadFailed,
				fmt.Errorf("读正文图 %q: %w", key, err))
		}
		url, err := s.mp.UploadContentImage(ctx, imgBytes)
		if err != nil {
			return nil, wrapMpErr(err)
		}
		imgURLs[c] = url
	}

	// 4. rows + daily（miss → nil 降级）→ 图片版 HTML → AddDraft。
	rows, err := s.fortunes.GetByDate(ctx, date)
	if err != nil {
		return nil, err
	}
	var daily *model.DailyHoroscope
	if s.daily != nil {
		// 同 BuildHTML：daily 读取钉 tid=0（contentCtx）。
		row, _, dErr := s.daily.GetByDate(contentCtx(ctx), date)
		if dErr != nil {
			return nil, dErr
		}
		daily = row
	}
	html := buildImageArticleHTML(daily, rows, date, func(c string) string { return imgURLs[c] })

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

// elementGroups 四象分组（顺序火→土→风→水，成员钉死——图片版软文 Global Constraints）。
var elementGroups = []struct {
	Name    string
	Members []string
}{
	{"火象星座", []string{"白羊座", "狮子座", "射手座"}},
	{"土象星座", []string{"金牛座", "处女座", "摩羯座"}},
	{"风象星座", []string{"双子座", "天秤座", "水瓶座"}},
	{"水象星座", []string{"巨蟹座", "天蝎座", "双鱼座"}},
}

// buildImageArticleHTML 图片版整版软文（spec 2026-11-16 §4.1）：日期徽章 → 重点星象/总述
// （daily 缺失降级跳过）→ 四象分组三张一行海报网格 → 星福柚关注区 → 免责小字。
// 全部 inline style + table（微信编辑器安全子集）；imgURL 由调用方定
// （预览=公开读相对路径，上传=微信 CDN URL）。纯函数，缺行整格跳过。
func buildImageArticleHTML(daily *model.DailyHoroscope, rows []*model.Fortune, date string, imgURL func(constellation string) string) string {
	fmap := make(map[string]*model.Fortune, len(rows))
	for _, f := range rows {
		if f != nil {
			fmap[f.Constellation] = f
		}
	}

	var b strings.Builder
	b.WriteString(`<meta charset="utf-8">`)
	b.WriteString(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; line-height: 1.8; color: #333; max-width: 680px; margin: 0 auto; padding: 0;">`)

	// 1. 日期徽章（YYYY/MM/DD）。
	slashDate := strings.ReplaceAll(date, "-", "/")
	fmt.Fprintf(&b, `<div style="display: inline-block; border: 1.5px solid #F2A8A8; border-radius: 8px; padding: 6px 16px; color: #E86A6A; font-size: 16px; font-weight: bold; margin-bottom: 16px;">%s</div>`, slashDate)

	// 2. 重点星象（非空才画）。
	if daily != nil && daily.Astro != "" {
		fmt.Fprintf(&b, `<p style="font-size: 17px; font-weight: bold; color: #333; margin: 12px 0;">重点星象：%s</p>`, daily.Astro)
	}
	// 3. 开篇总述（非空才画）。
	if daily != nil && daily.Intro != "" {
		fmt.Fprintf(&b, `<p style="font-size: 16px; color: #555; margin: 12px 0; line-height: 1.9;">%s</p>`, daily.Intro)
	}

	// 4. 四象分组：粉底胶囊 + 一行三格海报。
	for _, g := range elementGroups {
		b.WriteString(`<div style="text-align: center; margin: 28px 0 14px 0;">`)
		fmt.Fprintf(&b, `<span style="background: #FDEEF0; color: #E86A6A; border-radius: 999px; padding: 8px 28px; font-size: 18px; font-weight: bold;">%s</span>`, g.Name)
		b.WriteString(`</div>`)
		// border="0" + border:0：微信会给缺省属性的表格补默认单元格框线（用户反馈「图片有边框」）。
		b.WriteString(`<table width="100%" border="0" cellpadding="0" cellspacing="0" style="border-collapse: collapse; border: none;"><tr>`)
		for _, c := range g.Members {
			if _, ok := fmap[c]; !ok {
				continue // 缺行整格跳过（微信表格自然塌缩）
			}
			// 2026-11-16 用户反馈：格上不再画星座名/日期范围小标（海报自带大星座名），
			// 且收窄格距——海报图直接铺满 td。
			b.WriteString(`<td width="33%" style="border: 0; padding: 2px; vertical-align: top; text-align: center;">`)
			fmt.Fprintf(&b, `<img src="%s" style="width: 100%%; border: 0; border-radius: 8px; display: block;" alt="%s">`, imgURL(c), c)
			b.WriteString(`</td>`)
		}
		b.WriteString(`</tr></table>`)
	}

	// 5. 底部关注区（星福柚）。
	b.WriteString(`<div style="border: 1.5px solid #F2A8A8; border-radius: 12px; padding: 20px; margin: 32px 0 16px 0; text-align: center;">`)
	b.WriteString(`<p style="font-size: 16px; color: #333; font-weight: bold; margin: 6px 0;">运势早知道，好运跑不了。</p>`)
	b.WriteString(`<p style="font-size: 16px; color: #E86A6A; font-weight: bold; margin: 6px 0;">关注星福柚，每天解锁今日好运~</p>`)
	b.WriteString(`</div>`)

	// 6. 免责小字（沿用旧版语气）。
	b.WriteString(`<p style="text-align: center; color: #999; font-size: 12px; margin: 12px 0;"><em>以上运势仅供参考，祝您拥有美好的一天！</em></p>`)
	b.WriteString(`</div>`)
	return b.String()
}

// generateFortuneArticleHTML 整版 12 星座文章（照 goLucky _generateFortuneArticleHTML）。
// 保留仅作历史对照——BuildHTML 整版已切 buildImageArticleHTML（2026-11-16 图片版）。
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
