package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// DraftRunner 次日软文草稿任务（jobs.starfuyu.upload_draft，每天 17:10）：
// 抓目标日运势（缺则 DeepSeek 补抓）→ 按日期单双选底图合成封面 → UploadToMP 进草稿箱。
type DraftRunner interface {
	// UploadDraft 全流程；任一步失败上抛（封面失败不发草稿）。date=YYYY-MM-DD。
	UploadDraft(ctx context.Context, date string) error
}

type draftRunner struct {
	fortunes   FortuneService
	articles   ArticleService
	coverDir   string                                                        // starfuyu.cover_dir（封面输出目录）
	scriptPath string                                                        // 合成脚本路径（默认 scripts/starfuyu_cover.py）
	runCover   func(ctx context.Context, date, bgName, outPath string) error // 注入点：单测替换，生产=exec python3
}

// NewDraftRunner 组装草稿任务服务。scriptPath 缺省 scripts/starfuyu_cover.py
// （服务进程 cwd=仓库根，与 ./data 相对配置同假设）。
func NewDraftRunner(fortunes FortuneService, articles ArticleService, cfg *config.Config) DraftRunner {
	coverDir, script := "", "scripts/starfuyu_cover.py"
	if cfg != nil {
		coverDir = cfg.Starfuyu.CoverDir
	}
	r := &draftRunner{fortunes: fortunes, articles: articles, coverDir: coverDir, scriptPath: script}
	r.runCover = r.execCover
	return r
}

// coverBGForDate 单双选底图（用户裁定 2026-11-17）：日期天数单数 → 粉色版，
// 双数 → 橙色版；非法日期报错。
func coverBGForDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", errs.New(errs.CodeParamInvalid, fmt.Errorf("cover 底图日期须为 YYYY-MM-DD: %q", date))
	}
	if t.Day()%2 == 1 {
		return "cover_bg.png", nil
	}
	return "cover_bg_2.png", nil
}

// TomorrowSH 给定时刻的上海日历日 +1 天（YYYY-MM-DD）。
func TomorrowSH(now time.Time) string {
	return now.In(shLocation()).AddDate(0, 0, 1).Format("2006-01-02")
}

// UploadDraft fetch → 合成封面 → 上传草稿；顺序与半成品保护见 draft_test。
func (r *draftRunner) UploadDraft(ctx context.Context, date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errs.New(errs.CodeParamInvalid, fmt.Errorf("UploadDraft date 须为 YYYY-MM-DD: %q", date))
	}
	// 1. 目标日运势（已有 12 行则跳过——00:10 的当日抓取/昨日预抓均幂等）。
	if _, err := r.fortunes.Fetch(ctx, date, false); err != nil {
		return err
	}
	// 2. 封面：单双底图 + 合成脚本（失败不上抛前不发草稿）。
	bgName, err := coverBGForDate(date)
	if err != nil {
		return err
	}
	if r.coverDir == "" {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("cover_dir 未配置（starfuyu.cover_dir）"))
	}
	if err := os.MkdirAll(r.coverDir, 0o755); err != nil {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("创建封面目录 %q: %w", r.coverDir, err))
	}
	outPath := filepath.Join(r.coverDir, date+".png")
	if err := r.runCover(ctx, date, bgName, outPath); err != nil {
		return err
	}
	// 3. 草稿箱（UploadToMP 内部读 cover_dir/{date}.png 作封面）。
	if _, err := r.articles.UploadToMP(ctx, date); err != nil {
		return err
	}
	return nil
}

// execCover 调 python3 合成脚本：脚本 ROOT 由自身 __file__ 解析，--bg/--out 传绝对路径，
// 不依赖进程 cwd。脚本缺失/非零退出 → 30502 指明。
func (r *draftRunner) execCover(ctx context.Context, date, bgName, outPath string) error {
	script, err := filepath.Abs(r.scriptPath)
	if err != nil {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("封面脚本路径 %q: %w", r.scriptPath, err))
	}
	if fi, err := os.Stat(script); err != nil || fi.IsDir() {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("封面合成脚本不存在: %s", script))
	}
	// 脚本在 <root>/scripts/ 下 → 仓库根 = 其父目录。
	root := filepath.Dir(filepath.Dir(script))
	bgPath := filepath.Join(root, "data", "starfuyu", bgName)
	outAbs, err := filepath.Abs(outPath)
	if err != nil {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("封面输出路径 %q: %w", outPath, err))
	}
	cmd := exec.CommandContext(ctx, "python3", script, date, "--bg", bgPath, "-o", outAbs)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errs.New(errs.CodeStarfuyuImageFailed,
			fmt.Errorf("python3 合成封面失败: %w, 输出: %s", err, strings.TrimSpace(string(out))))
	}
	if fi, err := os.Stat(outAbs); err != nil || fi.IsDir() || fi.Size() == 0 {
		return errs.New(errs.CodeStarfuyuImageFailed, fmt.Errorf("封面产物缺失或为空: %s", outAbs))
	}
	return nil
}
