package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// BaziCalculator 八字计算器：抽接口便于测试替换（stub），FortuneFetcher 同款。
type BaziCalculator interface {
	Calculate(ctx context.Context, birthDate, birthTime, birthPlace string) (*model.BaziInfo, error)
}

// BaziService 我的八字：限流 + DeepSeek 计算 + 幂等覆盖保存。
type BaziService interface {
	// Calculate 限流 → 调 calculator → Upsert 我的八字。
	Calculate(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error)
	// GetMy 我的八字；miss → 30504。
	GetMy(ctx context.Context, memberUserID uint64) (*model.Bazi, error)
	// UpdateMy 同 Calculate（覆盖保存）。
	UpdateMy(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error)
}

// BaziReq 计算入参：birth_date 必填（2006-01-02）；birth_time 可空（非空须 15:04:05 或 15:04）。
type BaziReq struct {
	BirthDate  string
	BirthTime  string
	BirthPlace string
}

type baziService struct {
	bazis  repository.BaziRepository
	limits LimitService
	calc   BaziCalculator
}

func NewBaziService(bazis repository.BaziRepository, limits LimitService, calc BaziCalculator) BaziService {
	return &baziService{bazis: bazis, limits: limits, calc: calc}
}

func (s *baziService) Calculate(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error) {
	// 校验在限流之前：坏请求不消耗当日额度（goLucky handler 先校验后计数的顺序）。
	if err := validateBaziReq(req); err != nil {
		return nil, err
	}
	if err := s.limits.Allow(ctx, memberUserID, TypeBaziCalculate); err != nil {
		return nil, err
	}
	info, err := s.calc.Calculate(ctx, req.BirthDate, req.BirthTime, req.BirthPlace)
	if err != nil {
		return nil, err // calculator 契约：API 类失败上抛 errs.CodeInternal（50000）
	}
	if info == nil {
		return nil, errs.New(errs.CodeInternal, fmt.Errorf("八字计算器返回空结果"))
	}
	return s.bazis.Upsert(ctx, memberUserID, &model.Bazi{
		BirthDate:  req.BirthDate,
		BirthTime:  req.BirthTime,
		BirthPlace: req.BirthPlace,
		BaziInfo:   *info, // JSON 由 BaziInfo valuer 落列
	})
}

func (s *baziService) UpdateMy(ctx context.Context, memberUserID uint64, req BaziReq) (*model.Bazi, error) {
	return s.Calculate(ctx, memberUserID, req)
}

func (s *baziService) GetMy(ctx context.Context, memberUserID uint64) (*model.Bazi, error) {
	row, err := s.bazis.GetByMember(ctx, memberUserID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errs.New(errs.CodeStarfuyuBaziNotFound, nil)
	}
	return row, nil
}

// validateBaziReq birth_date 必填 + 格式、birth_time 可空校验（goLucky handler 同口径）。
func validateBaziReq(req BaziReq) error {
	if req.BirthDate == "" {
		return errs.New(errs.CodeParamMissing, fmt.Errorf("birth_date 必填"))
	}
	if _, err := time.Parse("2006-01-02", req.BirthDate); err != nil {
		return errs.New(errs.CodeParamInvalid, fmt.Errorf("birth_date 须为 2006-01-02 格式: %w", err))
	}
	if req.BirthTime != "" {
		if _, err := time.Parse("15:04:05", req.BirthTime); err != nil {
			if _, err2 := time.Parse("15:04", req.BirthTime); err2 != nil {
				return errs.New(errs.CodeParamInvalid, fmt.Errorf("birth_time 须为 15:04:05 或 15:04: %w", err))
			}
		}
	}
	return nil
}

// deepSeekBaziCalculator DeepSeek 八字计算器：prompt/解析照 goLucky service/bazi.go。
// API 失败、未配置一律 errs.CodeInternal（50000，不占 305xx——controller ruling）。
type deepSeekBaziCalculator struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewDeepSeekBaziCalculator 从全局配置构造计算器；超时 <=0 取缺省 120s（goLucky 同款）。
func NewDeepSeekBaziCalculator(cfg *config.Config) BaziCalculator {
	timeout := 120 * time.Second
	var apiKey, baseURL, model string
	if cfg != nil {
		ds := cfg.Starfuyu.Deepseek
		apiKey, baseURL, model = ds.APIKey, ds.BaseURL, ds.Model
		if ds.Timeout > 0 {
			timeout = time.Duration(ds.Timeout) * time.Second
		}
	}
	return &deepSeekBaziCalculator{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *deepSeekBaziCalculator) Calculate(ctx context.Context, birthDate, birthTime, birthPlace string) (*model.BaziInfo, error) {
	if c.apiKey == "" {
		return nil, errs.New(errs.CodeInternal, fmt.Errorf("starfuyu: deepseek api_key 未配置"))
	}
	if c.baseURL == "" {
		return nil, errs.New(errs.CodeInternal, fmt.Errorf("starfuyu: deepseek base_url 未配置"))
	}
	info, err := c.callAPI(ctx, birthDate, birthTime, birthPlace)
	if err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return info, nil
}

// baziPrompt 照 goLucky service/bazi.go CalculateBazi 原样。
const baziPrompt = `你是一个专业的八字命理师。请根据用户提供的出生信息，计算并返回详细的生辰八字信息。

输入信息：
- 出生日期：%s
- 出生时间：%s
- 出生地点：%s

请返回一个纯JSON对象，不要包含任何其他文字或解释，格式如下：
{
  "year_gan": "天干（如：甲、乙、丙等）",
  "year_zhi": "地支（如：子、丑、寅等）",
  "month_gan": "天干",
  "month_zhi": "地支",
  "day_gan": "天干",
  "day_zhi": "地支",
  "hour_gan": "天干",
  "hour_zhi": "地支",
  "wuxing": {
    "year_wuxing": "五行（金/木/水/火/土）",
    "month_wuxing": "五行",
    "day_wuxing": "五行",
    "hour_wuxing": "五行",
    "total_wuxing": {
      "金": 2,
      "木": 1,
      "水": 1,
      "火": 2,
      "土": 2
    }
  },
  "solar_term": "节气名称（如：立春、雨水等）",
  "lunar_date": "农历日期（如：正月初一）",
  "zodiac": "生肖（如：鼠、牛、虎等）",
  "constellation": "星座（如：白羊座、金牛座等）"
}

请确保返回的是有效的JSON格式，且所有字段都已填写。`

func (c *deepSeekBaziCalculator) callAPI(ctx context.Context, birthDate, birthTime, birthPlace string) (*model.BaziInfo, error) {
	prompt := fmt.Sprintf(baziPrompt, birthDate, birthTime, birthPlace)

	body, err := json.Marshal(deepSeekRequest{
		Model:    c.model,
		Messages: []deepSeekMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败 (状态码: %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		var errorResp deepSeekResponse
		if json.Unmarshal(respBody, &errorResp) == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("API错误 (状态码: %d, 类型: %s): %s", resp.StatusCode, errorResp.Error.Type, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var dsResp deepSeekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}
	if len(dsResp.Choices) == 0 {
		return nil, fmt.Errorf("API返回空数据")
	}
	content := strings.TrimSpace(dsResp.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("API返回内容为空")
	}

	var info model.BaziInfo
	if err := json.Unmarshal([]byte(stripJSONFences(content)), &info); err != nil {
		return nil, fmt.Errorf("解析八字数据JSON失败: %w, 内容预览: %s", err, truncate(content, 200))
	}
	if err := validateBaziInfo(&info); err != nil {
		return nil, fmt.Errorf("八字数据验证失败: %w", err)
	}
	return &info, nil
}

// stripJSONFences 清理 LLM 响应里可能的 ```json / ``` 围栏（goLucky cleanJSONContent 同款）。
func stripJSONFences(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

// validateBaziInfo 四柱完整性校验（goLucky 同款）。
func validateBaziInfo(bazi *model.BaziInfo) error {
	if bazi.YearGan == "" || bazi.YearZhi == "" {
		return fmt.Errorf("年柱信息不完整")
	}
	if bazi.MonthGan == "" || bazi.MonthZhi == "" {
		return fmt.Errorf("月柱信息不完整")
	}
	if bazi.DayGan == "" || bazi.DayZhi == "" {
		return fmt.Errorf("日柱信息不完整")
	}
	if bazi.HourGan == "" || bazi.HourZhi == "" {
		return fmt.Errorf("时柱信息不完整")
	}
	return nil
}
