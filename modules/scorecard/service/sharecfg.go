package service

import (
	"context"
	"encoding/json"

	"alexGo-cloud/modules/system/model"
	syscfg "alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/tenant"
)

// 分享卡配置键（system_configs，租户 0）：后台「系统参数」页可视化编辑，
// 小程序经 GET /api/public/scorecard/share-config 读取，替换三页硬编码分享卡。
const (
	ShareConfigKeyIndex     = "scorecard.share.index"     // 首页 pages/index/index
	ShareConfigKeySimulator = "scorecard.share.simulator" // 座位模拟器 pages/fun/compass/simulator
	ShareConfigKeySalary    = "scorecard.share.salary"    // 工资记账 pages/fun/salary/index
)

// ShareCard 单页分享卡：微信转发卡片的 title + 缩略图。
// ImageURL 空串 = 不传 imageUrl（微信回退当前页截图）。
type ShareCard struct {
	Title    string `json:"title"`
	ImageURL string `json:"imageUrl"`
}

// ShareCards 三页分享卡，键 = 后缀（index/simulator/salary），与路由 key 后缀一致。
type ShareCards map[string]ShareCard

// shareCardDefaults 内置兜底：key 缺失 / value 非法 JSON 时逐页回退，
// 保证后台误删参数后分享功能行为不劣于硬编码时代。
var shareCardDefaults = ShareCards{
	"index":     {Title: "登录后开房记账 · 好友互通", ImageURL: "http://qiniucdn.vate.ren/dapaijizhang/logo.png"},
	"simulator": {Title: "座位模拟器 · 趣味方位参考 · 仅供娱乐", ImageURL: ""},
	"salary":    {Title: "工资到账计算器 · 填工资看入账", ImageURL: ""},
}

// ShareConfigService 三页分享卡读写：读走 system_configs（ConfigService），
// 写只发生在后台参数页（本服务只 Seed 缺省行，不覆盖管理员改过的值）。
type ShareConfigService struct {
	configs syscfg.ConfigService
}

// NewShareConfigService 构造（fx 注入 system.ConfigService——scorecard 恒与 system
// 同进程装配，见 alexgo-server/cmd/main.go）。micro 形态不装 scorecard，无悬空风险。
func NewShareConfigService(configs syscfg.ConfigService) *ShareConfigService {
	return &ShareConfigService{configs: configs}
}

// shareCfgName 参数行显示名（后台列表用）。
var shareCfgName = map[string]string{
	"index":     "分享卡 · 首页",
	"simulator": "分享卡 · 座位模拟器",
	"salary":    "分享卡 · 工资记账",
}

// shareKeyFor 后缀 → 完整参数键。
func shareKeyFor(page string) string {
	switch page {
	case "index":
		return ShareConfigKeyIndex
	case "simulator":
		return ShareConfigKeySimulator
	default:
		return ShareConfigKeySalary
	}
}

// Get 三页分享卡：按 ctx 租户读参数；miss 且非平台租户再落 tenant 0（后台参数写在
// 租户 0，公开端点无 claims——域名解析万一给出非 0 租户也不丢配置）；仍 miss 用内置默认。
func (s *ShareConfigService) Get(ctx context.Context) ShareCards {
	out := make(ShareCards, len(shareCardDefaults))
	for page := range shareCardDefaults {
		out[page] = s.getOne(ctx, page)
	}
	return out
}

func (s *ShareConfigService) getOne(ctx context.Context, page string) ShareCard {
	def := shareCardDefaults[page]
	key := shareKeyFor(page)

	c, err := s.configs.GetByKey(ctx, key)
	if err == nil && c == nil && tenant.TenantIDFromContext(ctx) != 0 {
		// 平台租户兜底：管理员后台写入恒在 token 租户（当前部署 = 0）。
		c, err = s.configs.GetByKey(tenant.WithTenantID(ctx, 0), key)
	}
	if err != nil || c == nil || c.Value == "" {
		return def
	}
	var card ShareCard
	if json.Unmarshal([]byte(c.Value), &card) != nil {
		return def // 非法 JSON 按缺省处理，不让一条坏参数挂掉整个分享接口
	}
	if card.Title == "" {
		card.Title = def.Title
	}
	return card
}

// Seed 幂等补建三个缺省参数行（OnStart 调用）：只补缺、不覆盖——管理员已改过的
// value 保持原样。ctx 必须钉 tenant 0（与后台参数页读写同租户）。
func (s *ShareConfigService) Seed(ctx context.Context) error {
	ctx = tenant.WithTenantID(ctx, 0)
	for page, def := range shareCardDefaults {
		c, err := s.configs.GetByKey(ctx, shareKeyFor(page))
		if err != nil {
			return err
		}
		if c != nil {
			continue
		}
		raw, err := json.Marshal(def)
		if err != nil {
			return err
		}
		remark := "分享卡 {title, imageUrl} JSON；后台参数页可编辑，小程序 GET /api/public/scorecard/share-config 读取"
		if _, err := s.configs.Create(ctx, &model.SystemConfig{
			Key:    shareKeyFor(page),
			Value:  string(raw),
			Name:   shareCfgName[page],
			Remark: remark,
		}); err != nil {
			return err
		}
	}
	return nil
}
