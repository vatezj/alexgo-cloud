package service

import (
	"context"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

// FishRow /dex/table 契约行。
type FishRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Rarity  string `json:"rarity"`
	Period  string `json:"period"`
	Depth   string `json:"depth"`
	Flavor  string `json:"flavor"`
}

// DexService 图鉴双路：Table 全量启用条目；Report max 合并（spec §4.3）。
type DexService struct {
	fishes repository.FishRepository
	dex    repository.DexRepository
}

func NewDexService(fishes repository.FishRepository, dex repository.DexRepository) *DexService {
	return &DexService{fishes: fishes, dex: dex}
}

// Table 只返回 status=1 条目。
func (s *DexService) Table(ctx context.Context) ([]FishRow, error) {
	all, err := s.fishes.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FishRow, 0, len(all))
	for _, f := range all {
		out = append(out, FishRow{
			ID: f.FishID, Name: f.Name, Rarity: f.Rarity,
			Period: f.Period, Depth: f.Depth, Flavor: f.Flavor,
		})
	}
	return out, nil
}

// Report max 合并上报 counts；未知 fish_id 由调用方过滤（repo 只 upsert）。
// 返回合并后点亮数（CountByPlayer）。
func (s *DexService) Report(ctx context.Context, playerID uint64, counts map[string]int) (int64, error) {
	// 未知 id 忽略：只保留图鉴表内
	filtered := make(map[string]int, len(counts))
	for fishID, n := range counts {
		if n <= 0 {
			continue
		}
		f, err := s.fishes.GetByFishID(ctx, fishID)
		if err != nil {
			return 0, err
		}
		if f == nil {
			continue
		}
		filtered[fishID] = n
	}
	if err := s.dex.MergeReport(ctx, playerID, filtered); err != nil {
		return 0, err
	}
	return s.dex.CountByPlayer(ctx, playerID)
}

// ListPlayer 玩家图鉴进度（测试/控制器辅助）。
func (s *DexService) ListPlayer(ctx context.Context, playerID uint64) ([]*model.Dex, error) {
	return s.dex.ListByPlayer(ctx, playerID)
}
