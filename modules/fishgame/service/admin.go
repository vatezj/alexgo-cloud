package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	errs "alexGo-cloud/pkg/errors"
)

// 枚举全集（spec §4.2 / §5：rarity normal|misc|rare|epic、depth 四选一、period 令牌组合）。
var (
	fishRaritySet = map[string]bool{"normal": true, "misc": true, "rare": true, "epic": true}
	fishDepthSet  = map[string]bool{"any": true, "shallow": true, "mid": true, "deep": true}
	periodToken   = map[string]bool{"any": true, "day": true, "dusk": true, "night": true}
	configTypeSet = map[string]bool{"luck": true, "fish_king": true}
)

// validPeriod 逗号组合的每个令牌都必须 ∈ {day,dusk,night,any}。
func validPeriod(p string) bool {
	if p == "" {
		return false
	}
	for _, tok := range strings.Split(p, ",") {
		if !periodToken[strings.TrimSpace(tok)] {
			return false
		}
	}
	return true
}

func validFishInput(in FishInput) error {
	if !fishRaritySet[in.Rarity] || !validPeriod(in.Period) || !fishDepthSet[in.Depth] {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	return nil
}

// FishInput admin 建/改图鉴条目入参。
type FishInput struct {
	FishID string `json:"fish_id"`
	Name   string `json:"name"`
	Rarity string `json:"rarity"`
	Period string `json:"period"`
	Depth  string `json:"depth"`
	Flavor string `json:"flavor"`
	Sort   int    `json:"sort"`
	Status int    `json:"status"`
}

// ConfigInput admin 建/改运营覆盖入参（luck / fish_king 共用一行形状）。
type ConfigInput struct {
	Type      string     `json:"type"`
	Period    string     `json:"period"`
	DepthBias float64    `json:"depth_bias"`
	FishID    string     `json:"fish_id"`
	Week      string     `json:"week"`
	StartAt   *time.Time `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Status    int        `json:"status"`
}

// PlayerAdminRow /players 分页行：账号字段 + 点亮数（spec §4.2）。
type PlayerAdminRow struct {
	ID          uint64    `json:"id"`
	OpenID      string    `json:"openid"`
	Nick        string    `json:"nick"`
	Dex         int64     `json:"dex"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// LitRow 玩家详情点亮明细行（fish_id → count）。
type LitRow struct {
	FishID string `json:"fish_id"`
	Count  int    `json:"count"`
}

// PlayerDetail 玩家详情：基本 + 点亮明细（count≥1）+ 最近 20 条事件。
type PlayerDetail struct {
	Player *model.Player       `json:"player"`
	Lit    []LitRow            `json:"lit"`
	Events []*model.TrackEvent `json:"events"`
}

// RankPos 玩家在当前榜口径中的次位（admin 客服排查用）。
type RankPos struct {
	Position int `json:"position"`
	Total    int `json:"total"`
}

// DailyStat 看板按日趋势行（近 7 日，(day,name) 分桶）。
type DailyStat struct {
	Day   string `json:"day"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// StatsOut 看板聚合（spec §4.9）。
type StatsOut struct {
	PlayerTotal int64              `json:"player_total"`
	PlayerToday int64              `json:"player_today"`
	EventCounts map[string]int64   `json:"event_counts"` // 近 7 日按 name
	Daily       []DailyStat        `json:"daily"`        // 按日趋势（day asc, name asc）
	Funnel      map[string]int64   `json:"funnel"`       // catch/empty/escape/broken
	FunnelRatio map[string]float64 `json:"funnel_ratio"` // 各结果 / 四类合计
	SignCount   int64              `json:"sign_count"`
	BasketCount int64              `json:"basket_count"`
	AdCount     int64              `json:"ad_count"`
}

// AdminService admin 五组端点的业务聚合（casbin 鉴权在中间件，业务侧只做领域校验）。
type AdminService struct {
	fishes  repository.FishRepository
	configs repository.ConfigRepository
	events  repository.TrackEventRepository
	players repository.PlayerRepository
	dex     repository.DexRepository
	friend  *FriendService // 次位复用双口径榜单
}

func NewAdminService(
	fishes repository.FishRepository,
	configs repository.ConfigRepository,
	events repository.TrackEventRepository,
	players repository.PlayerRepository,
	dex repository.DexRepository,
	friend *FriendService,
) *AdminService {
	return &AdminService{fishes: fishes, configs: configs, events: events,
		players: players, dex: dex, friend: friend}
}

// ---- fish ----

// ListFish 图鉴分页：rarity 空不过滤，status<0 不过（controller 缺省 -1）。
func (s *AdminService) ListFish(ctx context.Context, rarity string, status int, page, size int) ([]*model.Fish, int64, error) {
	return s.fishes.ListAdmin(ctx, rarity, status, page, size)
}

// CreateFish 新建：fish_id 查重（→ 30401）+ 枚举校验（→ 10001）。
// 重复用 30401 是 brief 认可的取舍：无专用冲突码，借「图鉴条目」段位。
func (s *AdminService) CreateFish(ctx context.Context, in FishInput) (*model.Fish, error) {
	if err := validFishInput(in); err != nil {
		return nil, err
	}
	exist, err := s.fishes.GetByFishID(ctx, in.FishID)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, errs.New(errs.CodeFishgameFishNotFound, nil)
	}
	f := &model.Fish{
		TenantID: 0, FishID: in.FishID, Name: in.Name, Rarity: in.Rarity,
		Period: in.Period, Depth: in.Depth, Flavor: in.Flavor,
		Sort: in.Sort, Status: in.Status,
	}
	if err := s.fishes.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// UpdateFish 更新（含 status 启停）：查无 → 30401，枚举同新建。
func (s *AdminService) UpdateFish(ctx context.Context, id uint64, in FishInput) (*model.Fish, error) {
	f, err := s.fishes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errs.New(errs.CodeFishgameFishNotFound, nil)
	}
	if err := validFishInput(in); err != nil {
		return nil, err
	}
	f.FishID, f.Name, f.Rarity = in.FishID, in.Name, in.Rarity
	f.Period, f.Depth, f.Flavor = in.Period, in.Depth, in.Flavor
	f.Sort, f.Status = in.Sort, in.Status
	if err := s.fishes.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteFish 删除：查无 → 30401；被 dex/事件引用 → 30402（改启停即可）。
func (s *AdminService) DeleteFish(ctx context.Context, id uint64) error {
	f, err := s.fishes.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if f == nil {
		return errs.New(errs.CodeFishgameFishNotFound, nil)
	}
	used, err := s.dex.ExistsFish(ctx, f.FishID)
	if err != nil {
		return err
	}
	if !used {
		if used, err = s.events.ExistsFishRef(ctx, f.FishID); err != nil {
			return err
		}
	}
	if used {
		return errs.New(errs.CodeFishgameFishReferenced, nil)
	}
	return s.fishes.Delete(ctx, id)
}

// ---- config ----

// ListConfig 运营覆盖分页：type 空不过滤。
func (s *AdminService) ListConfig(ctx context.Context, typ string, page, size int) ([]*model.Config, int64, error) {
	return s.configs.ListAdmin(ctx, typ, page, size)
}

// validateConfig 建/改共用：type 枚举；luck 的 period/depth_bias≤0.3；
// fish_king 的 fish_id 必须表内真实存在且启用（→ 30403）、week 必填。
func (s *AdminService) validateConfig(ctx context.Context, in ConfigInput) error {
	if !configTypeSet[in.Type] {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	switch in.Type {
	case "luck":
		if !validPeriod(in.Period) || in.DepthBias < 0 || in.DepthBias > 0.3 {
			return errs.New(errs.CodeParamInvalid, nil)
		}
	case "fish_king":
		if in.FishID == "" || in.Week == "" {
			return errs.New(errs.CodeParamInvalid, nil)
		}
		f, err := s.fishes.GetByFishID(ctx, in.FishID)
		if err != nil {
			return err
		}
		if f == nil || f.Status != 1 {
			return errs.New(errs.CodeFishgameFishKingInvalid, nil)
		}
	}
	return nil
}

// CreateConfig 新建覆盖（fish_king 校验 → 30403）。
func (s *AdminService) CreateConfig(ctx context.Context, in ConfigInput) (*model.Config, error) {
	if err := s.validateConfig(ctx, in); err != nil {
		return nil, err
	}
	c := &model.Config{
		TenantID: 0, Type: in.Type, Period: in.Period, DepthBias: in.DepthBias,
		FishID: in.FishID, Week: in.Week, StartAt: in.StartAt, EndAt: in.EndAt,
		Status: in.Status,
	}
	if err := s.configs.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateConfig 更新（含启停）：查无 → 30404。
func (s *AdminService) UpdateConfig(ctx context.Context, id uint64, in ConfigInput) (*model.Config, error) {
	c, err := s.configs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errs.New(errs.CodeFishgameConfigNotFound, nil)
	}
	if err := s.validateConfig(ctx, in); err != nil {
		return nil, err
	}
	c.Type, c.Period, c.DepthBias = in.Type, in.Period, in.DepthBias
	c.FishID, c.Week, c.StartAt, c.EndAt, c.Status = in.FishID, in.Week, in.StartAt, in.EndAt, in.Status
	if err := s.configs.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteConfig 删除：查无 → 30404。
func (s *AdminService) DeleteConfig(ctx context.Context, id uint64) error {
	c, err := s.configs.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if c == nil {
		return errs.New(errs.CodeFishgameConfigNotFound, nil)
	}
	return s.configs.Delete(ctx, id)
}

// ---- events ----

// ListEvents 埋点分页（name/player_id/起止时间筛选，id 倒序由 repo 保证）。
func (s *AdminService) ListEvents(ctx context.Context, name string, playerID uint64, startT, endT int64, page, size int) ([]*model.TrackEvent, int64, error) {
	return s.events.ListAdmin(ctx, name, playerID, startT, endT, page, size)
}

// ---- players ----

// ListPlayers openid/nick 模糊分页，行内补点亮数（只聚合当页，避免全表 count）。
func (s *AdminService) ListPlayers(ctx context.Context, keyword string, page, size int) ([]PlayerAdminRow, int64, error) {
	players, total, err := s.players.ListAdmin(ctx, keyword, page, size)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint64, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
	}
	counts, err := s.dex.CountMapByPlayers(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]PlayerAdminRow, 0, len(players))
	for _, p := range players {
		rows = append(rows, PlayerAdminRow{
			ID: p.ID, OpenID: p.OpenID, Nick: p.Nick, Dex: counts[p.ID],
			CreatedAt: p.CreatedAt, LastLoginAt: p.LastLoginAt,
		})
	}
	return rows, total, nil
}

// PlayerDetail 基本信息 + 点亮明细（count≥1）+ 最近 20 条事件；查无 → 30002。
// 玩家无专用错误码，复用 CodeUserNotFound（客服排查场景语义通）。
func (s *AdminService) PlayerDetail(ctx context.Context, id uint64) (*PlayerDetail, error) {
	p, err := s.players.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.New(errs.CodeUserNotFound, nil)
	}
	dexRows, err := s.dex.ListByPlayer(ctx, id)
	if err != nil {
		return nil, err
	}
	lit := make([]LitRow, 0, len(dexRows))
	for _, d := range dexRows {
		if d.Count >= 1 {
			lit = append(lit, LitRow{FishID: d.FishID, Count: d.Count})
		}
	}
	events, _, err := s.events.ListAdmin(ctx, "", id, 0, 0, 1, 20)
	if err != nil {
		return nil, err
	}
	return &PlayerDetail{Player: p, Lit: lit, Events: events}, nil
}

// PlayerRank 该玩家当前榜单位次（复用 §4.1 双口径，复用 rankEntries 找位置）。
func (s *AdminService) PlayerRank(ctx context.Context, id uint64) (*RankPos, error) {
	p, err := s.players.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.New(errs.CodeUserNotFound, nil)
	}
	entries, err := s.friend.rankEntries(ctx, p)
	if err != nil {
		return nil, err
	}
	for i, e := range entries {
		if e.PlayerID == id {
			return &RankPos{Position: i + 1, Total: len(entries)}, nil
		}
	}
	// 理论不可达（rankEntries 必含本人）；兜底给出总长、次位 0 供排查。
	return &RankPos{Position: 0, Total: len(entries)}, nil
}

// ---- stats ----

// funnelNames 四类钓鱼结果（spec §4.9）。
var funnelNames = []string{"catch", "empty", "escape", "broken"}

// Stats 看板聚合：窗口 [now-7d, now]；按日分桶用本地时区（time.Unix → Local）。
// 聚合在内存完成（见 ListNameTSince 注释：窗口有界、跨库无方言差异）。
func (s *AdminService) Stats(ctx context.Context, now time.Time) (*StatsOut, error) {
	total, err := s.players.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today, err := s.players.CountCreatedSince(ctx, dayStart)
	if err != nil {
		return nil, err
	}
	rows, err := s.events.ListNameTSince(ctx, now.AddDate(0, 0, -7).Unix())
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(rows))
	buckets := make(map[string]int64, len(rows)) // "day\x00name" → count
	for _, r := range rows {
		counts[r.Name]++
		day := time.Unix(r.T, 0).Format("2006-01-02")
		buckets[day+"\x00"+r.Name]++
	}

	daily := make([]DailyStat, 0, len(buckets))
	for k, n := range buckets {
		if i := strings.IndexByte(k, 0); i >= 0 {
			daily = append(daily, DailyStat{Day: k[:i], Name: k[i+1:], Count: n})
		}
	}
	sort.Slice(daily, func(i, j int) bool {
		if daily[i].Day != daily[j].Day {
			return daily[i].Day < daily[j].Day
		}
		return daily[i].Name < daily[j].Name
	})

	funnel := make(map[string]int64, len(funnelNames))
	var sum int64
	for _, n := range funnelNames {
		funnel[n] = counts[n]
		sum += funnel[n]
	}
	ratio := make(map[string]float64, len(funnelNames))
	for _, n := range funnelNames {
		if sum > 0 {
			ratio[n] = float64(funnel[n]) / float64(sum)
		} else {
			ratio[n] = 0
		}
	}

	return &StatsOut{
		PlayerTotal: total,
		PlayerToday: today,
		EventCounts: counts,
		Daily:       daily,
		Funnel:      funnel,
		FunnelRatio: ratio,
		SignCount:   counts["sign_in"],
		BasketCount: counts["basket_claim"],
		AdCount:     counts["ad_request"] + counts["ad_rewarded"] + counts["ad_skip"] + counts["ad_fail"],
	}, nil
}
