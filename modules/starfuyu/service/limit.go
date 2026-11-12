package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
)

// 限流类型（brief 钉死：bazi 计算 / 求签，值同 spec §5 type 枚举）。
const (
	TypeBaziCalculate  = model.RequestTypeBaziCalculate  // "bazi_calculate"
	TypeDivinationDraw = model.RequestTypeDivinationDraw // "divination_draw"
)

// LimitService 每日请求限流：按 (member, 上海日历日, type) 计数。
type LimitService interface {
	// Allow 校验并计数：CheckAndIncrement 拿新 count，新 count > dailyLimit → 30503
	// （不回滚计数，照 goLucky CheckAndIncrement 语义——增量已发生，保留到次日换 key）。
	Allow(ctx context.Context, memberUserID uint64, typ string) error
}

type limitService struct {
	limits    repository.RequestLimitRepository
	baziDaily int // bazi_calculate 日限
	drawDaily int // divination_draw 日限
}

// NewLimitService dailyLimit 从 cfg.Starfuyu.Limits 取，<=0 缺省 5/10（cfg nil 同缺省）。
func NewLimitService(limits repository.RequestLimitRepository, cfg *config.Config) LimitService {
	s := &limitService{limits: limits, baziDaily: 5, drawDaily: 10}
	if cfg != nil {
		if v := cfg.Starfuyu.Limits.BaziCalculateDaily; v > 0 {
			s.baziDaily = v
		}
		if v := cfg.Starfuyu.Limits.DivinationDrawDaily; v > 0 {
			s.drawDaily = v
		}
	}
	return s
}

// dailyLimit typ → 当日上限；未知 typ 返回 false。
func (s *limitService) dailyLimit(typ string) (int, bool) {
	switch typ {
	case TypeBaziCalculate:
		return s.baziDaily, true
	case TypeDivinationDraw:
		return s.drawDaily, true
	default:
		return 0, false
	}
}

func (s *limitService) Allow(ctx context.Context, memberUserID uint64, typ string) error {
	limit, ok := s.dailyLimit(typ)
	if !ok {
		return errs.New(errs.CodeParamInvalid, fmt.Errorf("未知的限流类型: %s", typ))
	}
	// key 日期钉上海日历日（决定 18：生产容器 UTC，不得跟随 time.Local / TZ）。
	n, err := s.limits.CheckAndIncrement(ctx, memberUserID, TodaySH(), typ)
	if err != nil {
		return err
	}
	if n > limit {
		return errs.New(errs.CodeStarfuyuLimitExceeded,
			fmt.Errorf("今日 %s 已达上限 %d 次（当前 %d）", typ, limit, n))
	}
	return nil
}
