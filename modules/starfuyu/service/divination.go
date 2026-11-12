package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// 签号范围与抽签随机源（brief 钉死常量）。
const (
	signMin = 1
	signMax = 100
)

// SignInterpreter 签文/解读生成器：抽接口便于测试替换（stub），FortuneFetcher 同款。
type SignInterpreter interface {
	// SignContent 按签号生成签文（仅 cache miss 时调用）。
	SignContent(ctx context.Context, number int) (string, error)
	// Interpret 结合八字/问题生成解读（draw 专用）。
	Interpret(ctx context.Context, info *model.BaziInfo, signContent, question, category string) (*DivinationResult, error)
}

// DivinationResult 解读结果（JSON 字段照 goLucky generateInterpretation 输出结构）。
type DivinationResult struct {
	Interpretation string `json:"interpretation"`
	LuckyDirection string `json:"lucky_direction"`
	LuckyColor     string `json:"lucky_color"`
	Advice         string `json:"advice"`
	Category       string `json:"category"`
}

// DivinationService 求签解签：签文按号缓存（决定 15）+ 限流抽签落库 + 历史分页。
type DivinationService interface {
	// GetSign 公开端点：cache hit 直读；miss → interpreter.SignContent → Create 缓存
	// （305xx 不用，interpreter 失败回落模板，兜底也写缓存）；number∉[1,100] → 10001。
	GetSign(ctx context.Context, number int) (string, error)
	// Draw 限流 → 取/建签文缓存 →（有八字则）解读 → 落 Divination 行。
	Draw(ctx context.Context, memberUserID uint64, question, category string) (*model.Divination, error)
	// History 我的求签历史分页（created_at 倒序）。
	History(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error)
}

type divinationService struct {
	cache  repository.SignCacheRepository
	divs   repository.DivinationRepository
	bazis  repository.BaziRepository
	limits LimitService
	interp SignInterpreter
	// rng 抽签随机源，契约同 rand.Intn（入参 n，返回 [0,n)）；缺省 rand.Intn，测试可注入。
	rng func(int) int
}

func NewDivinationService(cache repository.SignCacheRepository, divs repository.DivinationRepository,
	bazis repository.BaziRepository, limits LimitService, interp SignInterpreter) DivinationService {
	return &divinationService{
		cache:  cache,
		divs:   divs,
		bazis:  bazis,
		limits: limits,
		interp: interp,
		rng:    rand.Intn,
	}
}

// GetSign 公开端点签文：1–100 校验（越界 10001）→ 缓存直读；miss → interpreter 生成
// （失败/空串回落按 number 模板）→ 落缓存——兜底也写，保证同号稳定、二次零 LLM。
func (s *divinationService) GetSign(ctx context.Context, number int) (string, error) {
	if number < signMin || number > signMax {
		return "", errs.New(errs.CodeParamInvalid,
			fmt.Errorf("签号须在 %d-%d 之间: %d", signMin, signMax, number))
	}
	// 签文缓存是内容表：读写钉 tid=0（repository 注释交办 Task 6）。
	ctx = contentCtx(ctx)
	row, err := s.cache.Get(ctx, number)
	if err != nil {
		return "", err
	}
	if row != nil {
		return row.SignContent, nil
	}

	var content string
	if s.interp != nil {
		c, iErr := s.interp.SignContent(ctx, number)
		if iErr != nil {
			log.Printf("[Divination] SignContent 失败回落模板: number=%d err=%v", number, iErr)
		} else {
			content = strings.TrimSpace(c)
		}
	}
	if content == "" {
		content = defaultSignContent(number) // 模板兜底也写缓存（决定 15）
	}
	// Create 冲突不覆盖首签并回读已存在行：并发首访双方都拿权威签文（同号稳定）。
	cached, err := s.cache.Create(ctx, number, content)
	if err != nil {
		return "", err
	}
	if cached != nil {
		return cached.SignContent, nil
	}
	return content, nil
}

// Draw 限流 → 随机签号（可注入 rng）→ 经 GetSign 同路径取签文 → 有八字则实时解读
// （失败降级模板，不 fail 整个 draw）→ 落 Divination 行。
func (s *divinationService) Draw(ctx context.Context, memberUserID uint64, question, category string) (*model.Divination, error) {
	if err := s.limits.Allow(ctx, memberUserID, TypeDivinationDraw); err != nil {
		return nil, err
	}
	number := s.rng(signMax-signMin+1) + signMin
	content, err := s.GetSign(ctx, number)
	if err != nil {
		return nil, err
	}

	// 八字可选：查询失败按无八字降级（goLucky 同口径：err == nil && row != nil 才用）。
	var baziID *uint64
	var info *model.BaziInfo
	if row, bErr := s.bazis.GetByMember(ctx, memberUserID); bErr == nil && row != nil {
		baziID = &row.ID
		info = &row.BaziInfo
	}

	var result *DivinationResult
	if info != nil && s.interp != nil {
		r, iErr := s.interp.Interpret(ctx, info, content, question, category)
		if iErr != nil {
			// 解读失败降级：签文照落库，解读字段填短模板（controller ruling）。
			log.Printf("[Divination] Interpret 失败降级模板: number=%d err=%v", number, iErr)
		} else {
			result = r
		}
	}
	// 无八字 / 解读失败 / 字段缺失 → 组装降级模板（字段全非空）。
	result = fillDivinationResult(result, number, category)

	return s.divs.Create(ctx, &model.Divination{
		MemberUserID:   memberUserID,
		BaziID:         baziID,
		Question:       question,
		SignNumber:     number,
		SignContent:    content,
		Interpretation: result.Interpretation,
		Category:       result.Category,
		LuckyDirection: result.LuckyDirection,
		LuckyColor:     result.LuckyColor,
		Advice:         result.Advice,
	})
}

// History 我的求签历史分页（page/size 兜底由 repo.Paginate 防御，size≤100 归 controller）。
func (s *divinationService) History(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error) {
	return s.divs.ListByMember(ctx, memberUserID, page, size)
}

// --- 内置模板兜底（照 goLucky getDefaultSignContent，扩为按 number 变体） ---

// signTemplates 基础签文模板（goLucky 5 条扩为 12 条）。
var signTemplates = []string{
	"天时地利人和，万事皆可成。",
	"云开见月明，前路自光明。",
	"积善之家必有余庆，行善之人必有善报。",
	"山重水复疑无路，柳暗花明又一村。",
	"宝剑锋从磨砺出，梅花香自苦寒来。",
	"欲穷千里目，更上一层楼。",
	"不积小流，无以成江海；不积跬步，无以至千里。",
	"静以修身，俭以养德；行稳致远，进而有为。",
	"塞翁失马，焉知非福；顺其自然，水到渠成。",
	"海纳百川，有容乃大；厚德载物，自强不息。",
	"春风得意马蹄疾，一日看尽长安花。",
	"守得云开见月明，静待花开终有时。",
}

// signDirections / signColors 降级解读的方位/颜色变体（按签号轮转）。
var (
	signDirections = []string{"东方", "南方", "西方", "北方", "中央"}
	signColors     = []string{"红色", "蓝色", "绿色", "金色", "紫色"}
)

// variantIdx 按签号取模板下标（signMin 起算，负数防御）。
func variantIdx(number, length int) int {
	if length <= 0 {
		return 0
	}
	i := (number - signMin) % length
	if i < 0 {
		i += length
	}
	return i
}

// defaultSignContent 模板兜底签文：基础模板 + 签号前缀，1–100 号内容互不相同（按 number 变体）。
func defaultSignContent(number int) string {
	return fmt.Sprintf("第%d签：%s", number, signTemplates[variantIdx(number, len(signTemplates))])
}

// normalizeCategory 解读类别兜底：got → 入参 category → 综合。
func normalizeCategory(got, want string) string {
	if got != "" {
		return got
	}
	if want != "" {
		return want
	}
	return "综合"
}

// degradeDivinationResult 解读降级模板：字段全非空、按签号变体（同号稳定）。
func degradeDivinationResult(number int, category string) *DivinationResult {
	return &DivinationResult{
		Interpretation: fmt.Sprintf(
			"第%d签签文平和，吉凶相济。解读服务暂不可用，单看此签：宜守正待时、稳中求进，所问之事顺其自然可成。", number),
		LuckyDirection: signDirections[variantIdx(number, len(signDirections))],
		LuckyColor:     signColors[variantIdx(number, len(signColors))],
		Advice:         "谋定而后动，勿急勿躁；宜整理旧务、蓄势待发，好事多磨终可成。",
		Category:       normalizeCategory("", category),
	}
}

// fillDivinationResult 解读结果装配：nil → 整组模板；非 nil 则空字段补模板、类别兜底
// （解读失败/缺字段不 fail 整个 draw——controller ruling）。
func fillDivinationResult(r *DivinationResult, number int, category string) *DivinationResult {
	d := degradeDivinationResult(number, category)
	if r == nil {
		return d
	}
	out := *r
	if strings.TrimSpace(out.Interpretation) == "" {
		out.Interpretation = d.Interpretation
	}
	if out.LuckyDirection == "" {
		out.LuckyDirection = d.LuckyDirection
	}
	if out.LuckyColor == "" {
		out.LuckyColor = d.LuckyColor
	}
	if out.Advice == "" {
		out.Advice = d.Advice
	}
	out.Category = normalizeCategory(out.Category, category)
	return &out
}

// --- DeepSeek 实现 ---

// deepSeekSignInterpreter DeepSeek 签文/解读生成器：prompt/解析照 goLucky
// service/divination.go；API 类失败一律 errs.CodeInternal（50000，不占 305xx）。
type deepSeekSignInterpreter struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewDeepSeekSignInterpreter 从全局配置构造；超时 <=0 取缺省 120s（Task 5 同款）。
func NewDeepSeekSignInterpreter(cfg *config.Config) SignInterpreter {
	timeout := 120 * time.Second
	var apiKey, baseURL, model string
	if cfg != nil {
		ds := cfg.Starfuyu.Deepseek
		apiKey, baseURL, model = ds.APIKey, ds.BaseURL, ds.Model
		if ds.Timeout > 0 {
			timeout = time.Duration(ds.Timeout) * time.Second
		}
	}
	return &deepSeekSignInterpreter{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

// signContentPrompt 照 goLucky service/divination.go getSignContent。
const signContentPrompt = `你是一个专业的签文生成器。请为第%d号签生成签文内容。

签文要求：
1. 签文应该富有哲理，寓意深刻
2. 内容应该积极正面，给人希望和指引
3. 字数控制在50-100字之间
4. 风格应该符合传统签文的特色

请直接返回签文内容，不要包含任何其他文字或格式。`

// interpretPrompt 照 goLucky service/divination.go generateInterpretation。
const interpretPrompt = `你是一个专业的解签大师。请根据用户的生辰八字和求签问题，为签文提供详细的解读。

用户八字信息（可选）：
%s

签文内容：
%s

求签问题（可选）：
%s

签文类别：%s

请返回一个纯JSON对象，不要包含任何其他文字或解释，格式如下：
{
  "interpretation": "详细的签文解读，包括运势分析、建议等，字数控制在200-300字",
  "category": "签文类别（事业/爱情/健康/财运/综合）",
  "lucky_direction": "幸运方位（如：东方、南方等）",
  "lucky_color": "幸运颜色（如：红色、蓝色等）",
  "advice": "具体建议，字数控制在50-100字"
}

请确保返回的是有效的JSON格式，且所有字段都已填写。`

func (i *deepSeekSignInterpreter) SignContent(ctx context.Context, number int) (string, error) {
	content, err := i.callAPI(ctx, fmt.Sprintf(signContentPrompt, number))
	if err != nil {
		return "", errs.New(errs.CodeInternal, err)
	}
	return content, nil
}

func (i *deepSeekSignInterpreter) Interpret(ctx context.Context, info *model.BaziInfo,
	signContent, question, category string) (*DivinationResult, error) {
	baziStr := "（无）"
	if info != nil {
		if b, err := json.Marshal(info); err == nil {
			baziStr = string(b)
		}
	}
	if category == "" {
		category = "综合"
	}
	raw, err := i.callAPI(ctx, fmt.Sprintf(interpretPrompt, baziStr, signContent, question, category))
	if err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	var out DivinationResult
	if err := json.Unmarshal([]byte(stripJSONFences(raw)), &out); err != nil {
		// 解析失败：原文即解读（goLucky 同口径），空字段由服务侧模板补齐。
		out.Interpretation = raw
	}
	return &out, nil
}

// callAPI 单次 HTTP 调用 DeepSeek（照 bazi calculator，不重试）。
func (i *deepSeekSignInterpreter) callAPI(ctx context.Context, prompt string) (string, error) {
	if i.apiKey == "" {
		return "", fmt.Errorf("starfuyu: deepseek api_key 未配置")
	}
	if i.baseURL == "" {
		return "", fmt.Errorf("starfuyu: deepseek base_url 未配置")
	}
	body, err := json.Marshal(deepSeekRequest{
		Model:    i.model,
		Messages: []deepSeekMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+i.apiKey)

	resp, err := i.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败 (状态码: %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		var errorResp deepSeekResponse
		if json.Unmarshal(respBody, &errorResp) == nil && errorResp.Error.Message != "" {
			return "", fmt.Errorf("API错误 (状态码: %d, 类型: %s): %s", resp.StatusCode, errorResp.Error.Type, errorResp.Error.Message)
		}
		return "", fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var dsResp deepSeekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return "", fmt.Errorf("解析响应JSON失败: %w", err)
	}
	if len(dsResp.Choices) == 0 {
		return "", fmt.Errorf("API返回空数据")
	}
	content := strings.TrimSpace(dsResp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("API返回内容为空")
	}
	return content, nil
}
