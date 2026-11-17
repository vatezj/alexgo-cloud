package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/config"
)

// FortuneFetcher 每日运势抓取器：一次调用必须恰好返回 12 星座，否则 error（无部分落库）。
// 抽接口便于测试替换（stub）。
type FortuneFetcher interface {
	FetchFortunes(ctx context.Context, date string) ([]*model.Fortune, *model.DailyHoroscope, error)
}

// deepSeekFetcher 真实现：POST cfg.Starfuyu.Deepseek.BaseURL，prompt 照 goLucky service/deepseek.go。
type deepSeekFetcher struct {
	apiKey     string
	baseURL    string
	model      string
	maxRetries int
	retryDelay time.Duration
	client     *http.Client
}

// NewDeepSeekFetcher 从全局配置构造抓取器；超时/重试 <=0 取缺省 120s / 3 次 / 2s。
func NewDeepSeekFetcher(cfg *config.Config) FortuneFetcher {
	timeout := 120 * time.Second
	maxRetries := 3
	retryDelay := 2 * time.Second
	var apiKey, baseURL, model string
	if cfg != nil {
		ds := cfg.Starfuyu.Deepseek
		apiKey, baseURL, model = ds.APIKey, ds.BaseURL, ds.Model
		if ds.Timeout > 0 {
			timeout = time.Duration(ds.Timeout) * time.Second
		}
		if ds.MaxRetries > 0 {
			maxRetries = ds.MaxRetries
		}
		if ds.RetryDelay > 0 {
			retryDelay = time.Duration(ds.RetryDelay) * time.Second
		}
	}
	return &deepSeekFetcher{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		client:     &http.Client{Timeout: timeout},
	}
}

type deepSeekRequest struct {
	Model    string            `json:"model"`
	Messages []deepSeekMessage `json:"messages"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message deepSeekMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// FetchFortunes 一次 HTTP 调用拉 12 星座；网络类错误按 cfg 重试，其余错误不重试
// （分类照 goLucky service/deepseek.go）。
func (f *deepSeekFetcher) FetchFortunes(ctx context.Context, date string) ([]*model.Fortune, *model.DailyHoroscope, error) {
	if f.apiKey == "" {
		return nil, nil, fmt.Errorf("starfuyu: deepseek api_key 未配置")
	}
	if f.baseURL == "" {
		return nil, nil, fmt.Errorf("starfuyu: deepseek base_url 未配置")
	}

	var lastErr error
	for attempt := 1; attempt <= f.maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("[DeepSeek] 第 %d 次重试获取运势数据 (日期: %s)", attempt, date)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(f.retryDelay):
			}
		}

		fortunes, daily, err := f.callAPI(ctx, date)
		if err == nil {
			log.Printf("[DeepSeek] 成功获取运势数据 (日期: %s, 星座数量: %d, daily: %v)", date, len(fortunes), daily != nil)
			return fortunes, daily, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}
	return nil, nil, fmt.Errorf("获取运势数据失败 (日期: %s): %w", date, lastErr)
}

// isRetryable 仅网络/超时类错误重试（照 goLucky）。
func isRetryable(err error) bool {
	msg := err.Error()
	for _, kw := range []string{"timeout", "deadline exceeded", "connection", "network", "EOF"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func (f *deepSeekFetcher) callAPI(ctx context.Context, date string) ([]*model.Fortune, *model.DailyHoroscope, error) {
	prompt := fmt.Sprintf(`您是一个专业的星座运势API服务。请根据我提供的日期，为所有12个星座生成对应的运势数据，并返回一个格式严格的JSON对象（对象包裹）。

### **角色与输入**
- **角色**：专业的星座运势数据生成器。
- **输入**：一个名为 date 的参数，其值为格式为 YYYY-MM-DD 的日期字符串（例如：%s）。这是生成运势的唯一依据。

### **输出规范**
1. **格式**：一个纯JSON对象（对象包裹），含 daily_astro、daily_intro 与 fortunes 数组；fortunes 含12个对象，每个对象对应一个星座的运势数据。
2. **纯净性**：响应必须只包含这个JSON对象，没有任何额外的解释、问候语、代码标记或上下文文本。
3. **顺序**：fortunes 数组中的对象顺序必须固定为：白羊座、金牛座、双子座、巨蟹座、狮子座、处女座、天秤座、天蝎座、射手座、摩羯座、水瓶座、双鱼座。

### **JSON结构**
顶层为一个对象（对象包裹）：
{
  "daily_astro": "当日重点星象一句，≤20字（如：月水星，月土六合，木火合）",
  "daily_intro": "开篇总述一段，≤60字（积极热情、覆盖金钱与感情好消息的基调）",
  "fortunes": [ /* 下方12个星座对象 */ ]
}
daily_astro/daily_intro 与星座无关、全篇同一条。

每个星座对象（fortunes 数组元素）必须严格遵循以下字段定义：
{
  "constellation": "星座中文名",
  "date": "YYYY-MM-DD",
  "daily_overview": "今天是YYYY年MM月DD日 星期X。今天[星座名]整体运势[用一句话概括]。爱情运势X星，[一句话说明]。事业运势X星，[一句话说明]。财务运势X星，[一句话说明]。健康运势X星，[一句话说明]。幸运颜色是X，幸运数字是X。",
  "overall_score": 75,
  "love_score": 80,
  "career_score": 70,
  "wealth_score": 65,
  "health_score": 85,
  "lucky_color": {
    "chinese": "红色",
    "english": "Red",
    "hex": "#FF0000"
  },
  "lucky_number": 7,
  "summary": "今日运势平稳，宜规划未来。",
  "love_tip": "多与伴侣进行深度交流。",
  "career_tip": "处理文件时请仔细核对。",
  "wealth_tip": "避免进行高风险的投资。",
  "health_tip": "注意规律作息，补充水分。",
  "match_constellation": "合拍星座中文名（12星座之一，不得等于本星座自身）",
  "advice": "今日建议（≤12字）",
  "avoid": "今日避免（≤9字）",
  "reunion_tip": "复合运简述（≤14字）",
  "single_tip": "脱单运简述（≤14字）",
  "job_tip": "求职运简述（≤14字）",
  "reunion_score": 70,
  "single_score": 75,
  "job_score": 65,
  "daily_todos": ["必做一（≤14字）", "必做二（≤14字）", "必做三（≤14字）"],
  "slogan": "由当日运势生发的口号（呼应 overall_score 高低与 summary/advice 主题），≤16字"
}

### **生成要求**
1. 所有分数应为0-100的整数。
2. 运势描述（summary，*_tip）和daily_overview需基于日期和星座生成，确保内容每日不同且合理。
3. 幸运色和幸运数字应具备一定的多样性，避免全部相同。
4. 在生成 daily_overview 时，需要根据当前星座的运势趋势，确定该星座的爱情、事业、财务、健康运势的星级（1-5星），然后为每个运势生成一句简短的说明。
5. match_constellation 不得等于本星座自身（必须是12星座之一）。
6. advice/avoid 每日不同。
7. summary ≤24字。
8. slogan 为卡片底部横幅口号：必须由当日运势生发——先看该星座 overall_score 高低与 summary/advice 的主题，高分写乘势/庆祝、低分写蓄力/从容，落到与 summary 同一件事上；积极向上、每日不同、≤16字。
9. daily_astro/daily_intro 是当日全局一条：与12星座无关、全篇同一条，不随星座变化。
10. daily_intro 不得用「今天/今日」（文章提前一天发，口吻用「明天/明日」或中性「这天」）；其余星座文案（summary/tip/slogan 等）仍按当日口吻写（与海报模板「今日」标签配套）。

请严格遵循以上规则。现在，请根据日期 %s 生成对象包裹（daily_astro、daily_intro + fortunes 内12个星座详细数据）的运势信息。`, date, date)

	body, err := json.Marshal(deepSeekRequest{
		Model:    f.model,
		Messages: []deepSeekMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应体失败 (状态码: %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		var errorResp deepSeekResponse
		if json.Unmarshal(respBody, &errorResp) == nil && errorResp.Error.Message != "" {
			return nil, nil, fmt.Errorf("API错误 (状态码: %d, 类型: %s): %s", resp.StatusCode, errorResp.Error.Type, errorResp.Error.Message)
		}
		return nil, nil, fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var dsResp deepSeekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}
	if len(dsResp.Choices) == 0 {
		return nil, nil, fmt.Errorf("API返回空数据")
	}
	content := strings.TrimSpace(dsResp.Choices[0].Message.Content)
	if content == "" {
		return nil, nil, fmt.Errorf("API返回内容为空")
	}

	// 对象包裹优先（daily_astro/daily_intro + fortunes）；失败回退纯数组（模型旧格式，
	// daily=nil 不判失败——文章降级少两块，spec §3.2 容错）。
	var wrap struct {
		DailyAstro string           `json:"daily_astro"`
		DailyIntro string           `json:"daily_intro"`
		Fortunes   []*model.Fortune `json:"fortunes"`
	}
	var fortunes []*model.Fortune
	var daily *model.DailyHoroscope
	if err := json.Unmarshal([]byte(content), &wrap); err == nil && len(wrap.Fortunes) > 0 {
		fortunes = wrap.Fortunes
		daily = &model.DailyHoroscope{Astro: wrap.DailyAstro, Intro: wrap.DailyIntro}
	} else if err := json.Unmarshal([]byte(content), &fortunes); err != nil {
		return nil, nil, fmt.Errorf("解析运势数据JSON失败: %w, 内容预览: %s", err, truncate(content, 200))
	}
	if len(fortunes) != 12 {
		return nil, nil, fmt.Errorf("运势数据不完整，期望12个，实际%d个", len(fortunes))
	}
	// 日期/租户由 service 侧钉死（内容表 tid=0），这里同样置位保证直接可用。
	// JSON `[null, {...}×11]` 能过 len==12，nil 行必须先挡（否则 row.TenantID panic）。
	for _, row := range fortunes {
		if row == nil {
			return nil, nil, fmt.Errorf("运势数据含空行")
		}
		row.TenantID = 0
		row.Date = date
		// 合拍星座非 12 枚举（含 DeepSeek 幻觉/漏字段空串）→ 存空串，不判失败。
		if !IsValidConstellation(row.MatchConstellation) {
			row.MatchConstellation = ""
		}
	}
	return fortunes, daily, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
