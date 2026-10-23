package service

import (
	"context"
	"fmt"
	"time"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

const (
	basketRateSec = 2400     // 40min / bait
	basketCapSec  = 8 * 3600 // 8h 封顶
	basketMaxBait = 12
)

// DailyService 每日循环：config/daily、daily/sign、basket（spec §4.1）。
type DailyService struct {
	configs repository.ConfigRepository
	fishes  repository.FishRepository
	signs   repository.SignRepository
}

func NewDailyService(
	configs repository.ConfigRepository,
	fishes repository.FishRepository,
	signs repository.SignRepository,
) *DailyService {
	return &DailyService{configs: configs, fishes: fishes, signs: signs}
}

// DailyConfigResponse /config/daily 契约形状（裸 JSON）。
type DailyConfigResponse struct {
	Luck struct {
		Period    string  `json:"period"`
		DepthBias float64 `json:"depth_bias"`
	} `json:"luck"`
	FishKing struct {
		FishID string `json:"fish_id"`
		Week   string `json:"week"`
	} `json:"fish_king"`
}

// DailyConfig 运营覆盖优先，否则确定性自算（fish_id 二次过滤：非法覆盖回退自算）。
func (s *DailyService) DailyConfig(ctx context.Context, now time.Time) (*DailyConfigResponse, error) {
	var out DailyConfigResponse

	// luck：覆盖或自算
	luckCfg, err := s.configs.ListActiveByType(ctx, "luck", now)
	if err != nil {
		return nil, err
	}
	epochDays := YMDToDays(now.Year(), int(now.Month()), now.Day())
	period, bias := ComputeLuck(epochDays)
	if luckCfg != nil && luckCfg.Period != "" {
		switch luckCfg.Period {
		case "day", "dusk", "night", "any":
			period = luckCfg.Period
		}
		if luckCfg.DepthBias >= 0 && luckCfg.DepthBias <= 0.3 {
			bias = luckCfg.DepthBias
		}
	}
	out.Luck.Period = period
	out.Luck.DepthBias = bias

	// fish_king：覆盖（必须表内真实启用 id）或自算
	week := ISOMonthWeek(now)
	pool, err := s.fishKingPool(ctx)
	if err != nil {
		return nil, err
	}
	fishID := ComputeFishKing(week, pool)
	fkCfg, err := s.configs.ListActiveByType(ctx, "fish_king", now)
	if err != nil {
		return nil, err
	}
	if fkCfg != nil && fkCfg.FishID != "" {
		f, err := s.fishes.GetByFishID(ctx, fkCfg.FishID)
		if err != nil {
			return nil, err
		}
		// 二次过滤：条目存在且启用才下发覆盖；否则保留自算（spec §4.7）
		if f != nil && f.Status == 1 {
			fishID = f.FishID
		}
	}
	out.FishKing.FishID = fishID
	out.FishKing.Week = week
	return &out, nil
}

// fishKingPool 启用条目中 rare+epic，按 sort 插入序（确定性）。
func (s *DailyService) fishKingPool(ctx context.Context) ([]string, error) {
	all, err := s.fishes.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	var pool []string
	for _, f := range all {
		if f.Rarity == "rare" || f.Rarity == "epic" {
			pool = append(pool, f.FishID)
		}
	}
	return pool, nil
}

// Sign 签到对账：同日重复 ok=false；否则服务端按上次 date 差值权威算 streak，
// 奖 bait 表 1/2/3→+1/+2/+3、4-6→+3、7+→+5。date 必须 YYYY-MM-DD。
func (s *DailyService) Sign(ctx context.Context, playerID uint64, date string) (ok bool, bait int, err error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return false, 0, fmt.Errorf("bad date: %w", err)
	}
	exist, err := s.signs.GetByPlayerDate(ctx, playerID, date)
	if err != nil {
		return false, 0, err
	}
	if exist != nil {
		return false, 0, nil
	}
	streak := 1
	last, err := s.signs.GetLastByPlayer(ctx, playerID)
	if err != nil {
		return false, 0, err
	}
	if last != nil {
		lastT, perr := time.Parse("2006-01-02", last.Date)
		curT, perr2 := time.Parse("2006-01-02", date)
		if perr == nil && perr2 == nil {
			diff := int(curT.Sub(lastT).Hours() / 24)
			if diff == 1 {
				streak = last.Streak + 1
			}
			// diff >= 2 或 diff <= 0（乱序）→ 归 1
		}
	}
	reward := signReward(streak)
	if err := s.signs.Create(ctx, &model.Sign{
		TenantID: 0, PlayerID: playerID, Date: date, Streak: streak, Bait: reward,
	}); err != nil {
		return false, 0, err
	}
	return true, reward, nil
}

// signReward 连击 → bait（契约表）。
func signReward(streak int) int {
	switch {
	case streak == 1:
		return 1
	case streak == 2:
		return 2
	case streak == 3:
		return 3
	case streak >= 4 && streak <= 6:
		return 3
	default:
		return 5
	}
}

// Basket 纯计算：elapsed = now-since（负钳 0）→ bait=min(elapsed/2400, 12)，
// capped = elapsed >= 8h。不落表（spec 决定 9）。
func (s *DailyService) Basket(sinceUnix int64) (bait int, capped bool, err error) {
	now := time.Now().Unix()
	elapsed := now - sinceUnix
	if elapsed < 0 {
		elapsed = 0
	}
	bait = int(elapsed / basketRateSec)
	if bait > basketMaxBait {
		bait = basketMaxBait
	}
	capped = elapsed >= basketCapSec
	return bait, capped, nil
}
