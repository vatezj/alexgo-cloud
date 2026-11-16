package service

import (
	"context"
	"fmt"
	"log"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/modules/starfuyu/repository"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

// FortuneService 每日星座运势：抓取补缺（一次 12 星座、只补缺、force 覆盖）+ 内容读取。
type FortuneService interface {
	// Fetch 拉取并落库：已有 count==12 且 !force → (skipped=true, nil)；
	// force 或不足 12 → FetchFortunes + UpsertBatch（整批，失败不落）。
	// 抓取/校验/落库失败 → 30506。
	Fetch(ctx context.Context, date string, force bool) (skipped bool, err error)
	// GetByDate 当日全部运势（tid=0 读）；无数据空列表非 error。
	GetByDate(ctx context.Context, date string) ([]*model.Fortune, error)
	// GetOne 单星座运势；miss → 30501。
	GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error)
	// ListAdmin 管理端分页（tid=0 读）。
	ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error)
}

type fortuneService struct {
	fortunes repository.FortuneRepository
	daily    repository.DailyRepository
	fetcher  FortuneFetcher
}

func NewFortuneService(fortunes repository.FortuneRepository, daily repository.DailyRepository, fetcher FortuneFetcher) FortuneService {
	return &fortuneService{fortunes: fortunes, daily: daily,
		fetcher: fetcher}
}

// contentCtx 内容表读写钉 tid=0（决定 6：内容不随租户）。
func contentCtx(ctx context.Context) context.Context {
	return tenant.WithTenantID(ctx, 0)
}

func (s *fortuneService) Fetch(ctx context.Context, date string, force bool) (bool, error) {
	ctx = contentCtx(ctx)

	n, err := s.fortunes.CountByDate(ctx, date)
	if err != nil {
		return false, errs.New(errs.CodeStarfuyuFetchFailed, fmt.Errorf("统计当日运势失败: %w", err))
	}
	if n == 12 && !force {
		return true, nil // 已满且非强制：不调抓取器
	}

	rows, daily, err := s.fetcher.FetchFortunes(ctx, date)
	if err != nil {
		return false, errs.New(errs.CodeStarfuyuFetchFailed, err)
	}
	if len(rows) != 12 {
		return false, errs.New(errs.CodeStarfuyuFetchFailed,
			fmt.Errorf("运势数据不完整，期望12个，实际%d个", len(rows)))
	}
	// nil 行 + 星座合法性/唯一性校验：`[null,…]` 过 len 检查后 row.TenantID 会
	// panic；重复/未知星座过 len 但 UpsertBatch 静默折叠 → CountByDate 永远 <12。
	// 任一不合法 → 30506、零落库。
	seen := make(map[string]struct{}, 12)
	for _, row := range rows {
		if row == nil {
			return false, errs.New(errs.CodeStarfuyuFetchFailed,
				fmt.Errorf("运势数据含空行"))
		}
		if !IsValidConstellation(row.Constellation) {
			return false, errs.New(errs.CodeStarfuyuFetchFailed,
				fmt.Errorf("非法星座名: %q", row.Constellation))
		}
		if _, dup := seen[row.Constellation]; dup {
			return false, errs.New(errs.CodeStarfuyuFetchFailed,
				fmt.Errorf("星座重复: %q", row.Constellation))
		}
		seen[row.Constellation] = struct{}{}
		row.TenantID = 0 // 内容表恒 0
		row.Date = date  // 以入参日期为准
	}
	if err := s.fortunes.UpsertBatch(ctx, rows); err != nil {
		return false, errs.New(errs.CodeStarfuyuFetchFailed, fmt.Errorf("运势整批落库失败: %w", err))
	}
	// 日级星象/总述：增强数据——失败仅记日志，不阻断抓取（spec §3.3 裁定，
	// 文章侧 GetByDate miss 自然降级）。
	if daily != nil {
		daily.Date = date // 以入参日期为准
		if err := s.daily.Upsert(ctx, daily); err != nil {
			log.Printf("[Fortune] 日级星象落库失败 date=%s err=%v", date, err)
		}
	}
	return false, nil
}

func (s *fortuneService) GetByDate(ctx context.Context, date string) ([]*model.Fortune, error) {
	list, err := s.fortunes.GetByDate(contentCtx(ctx), date)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []*model.Fortune{}, nil
	}
	return list, nil
}

func (s *fortuneService) GetOne(ctx context.Context, date, constellation string) (*model.Fortune, error) {
	row, err := s.fortunes.GetOne(contentCtx(ctx), date, constellation)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errs.New(errs.CodeStarfuyuFortuneNotFound, nil)
	}
	return row, nil
}

func (s *fortuneService) ListAdmin(ctx context.Context, date, constellation string, page, size int) ([]*model.Fortune, int64, error) {
	return s.fortunes.ListAdmin(contentCtx(ctx), date, constellation, page, size)
}
