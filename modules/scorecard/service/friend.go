package service

import (
	"context"
	"sort"

	"gorm.io/gorm"

	membersvc "alexGo-cloud/modules/member/service"
	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/modules/scorecard/repository"
	errs "alexGo-cloud/pkg/errors"
)

// FriendItem 好友排名行（spec §4.1 GET /friends）：昵称头像批量补齐，
// total_score 降序展示；IsBlocked = 「我拉黑了对方」的单向行存在与否。
type FriendItem struct {
	UserID      uint64  `json:"user_id"`
	Nickname    string  `json:"nickname"`
	Avatar      string  `json:"avatar"`
	RankEnabled bool    `json:"rank_enabled"`
	TotalScore  int64   `json:"total_score"`
	WinRate     float64 `json:"win_rate"`
	IsBlocked   bool    `json:"is_blocked"`
}

// FriendService scorecard 好友服务：排名列表 / 加好友（双向拦截）/ 删除 /
// 拉黑（幂等）/ 排名参与开关 / 监管端分页。Task 12 app 控制器与 Task 13 监管控制器消费。
type FriendService interface {
	// List 我的好友排名：我方 friend 行 → briefs 批量 → 逐人 record 统计
	// （好友 ≤20 量级，循环即可）→ 按 total_score 降序（同分 user_id 升序）。
	List(ctx context.Context, userID uint64) ([]FriendItem, error)
	// Add 加好友：参数（自加/零值 10001）→ 对方 Brief 存在（30002）→
	// 已是好友（30109）→ 双向拉黑拦截（30108）→ 双向各写一行单事务（rank_enabled=1）。
	Add(ctx context.Context, userID, friendID uint64) error
	// Remove 删好友：双向删行单事务（与 Add 对称，两行同生同灭）；空删幂等。
	Remove(ctx context.Context, userID, friendID uint64) error
	// SetBlack 拉黑/解除：blocked=true 写我方单向行（幂等 upsert）；false 删行（幂等）。
	SetBlack(ctx context.Context, userID, blockedID uint64, blocked bool) error
	// SetRank 排名参与开关：只动我方指向 friendID 的单行（行不存在 = 无害空更新）。
	SetRank(ctx context.Context, userID, friendID uint64, enabled bool) error
	// AdminList 监管端好友关系分页（Task 13 消费，分页直查）。
	AdminList(ctx context.Context, page, size int) ([]*model.Friend, int64, error)
}

type friendService struct {
	db     *gorm.DB
	friend repository.FriendRepository
	black  repository.FriendBlackRepository
	record repository.RecordRepository // List 排名聚合用
	briefs membersvc.BriefProvider
}

// NewFriendService 构造好友服务（fx 装配，见 module.go：briefs 标 optional 同
// RoomService——micro 模式本进程不装 member.FxModule，图里无 BriefProvider 映射）。
func NewFriendService(db *gorm.DB, friend repository.FriendRepository,
	black repository.FriendBlackRepository, record repository.RecordRepository,
	briefs membersvc.BriefProvider) FriendService {
	return &friendService{db: db, friend: friend, black: black, record: record, briefs: briefs}
}

// validatePair 参数门禁：零值与自己对自己一律 10001（自加/自删/自拉黑/自开关均无意义）。
func validatePair(userID, otherID uint64) error {
	if userID == 0 || otherID == 0 || userID == otherID {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	return nil
}

// List 我的好友排名：ListByUser（租户插件隔离）→ briefs 批量（透传租户 ctx，T6 先例；
// 拉取失败降级空昵称，与 buildSnapshot 同策略）→ 逐人 StatsByUser + 我方拉黑行 →
// total_score 降序。同分按 user_id 升序保证稳定可断言。
func (s *friendService) List(ctx context.Context, userID uint64) ([]FriendItem, error) {
	if userID == 0 {
		return nil, errs.New(errs.CodeParamInvalid, nil)
	}
	rows, err := s.friend.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]FriendItem, 0, len(rows))
	ids := make([]uint64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.FriendUserID)
	}
	// 昵称头像批量补齐（失败降级留空；micro 模式 briefs 未装配同降级）。
	briefByID := make(map[uint64]membersvc.UserBrief, len(ids))
	if len(ids) > 0 && s.briefs != nil {
		if bs, err := s.briefs.GetBriefs(ctx, ids); err == nil {
			for _, b := range bs {
				briefByID[b.ID] = b
			}
		}
	}
	for _, r := range rows {
		total, win, lose, _, err := s.record.StatsByUser(ctx, r.FriendUserID)
		if err != nil {
			return nil, err
		}
		// IsBlocked 取我方单向行（spec §friend 对 List 未定义按拉黑过滤，只带标记）。
		blk, err := s.black.Get(ctx, userID, r.FriendUserID)
		if err != nil {
			return nil, err
		}
		item := FriendItem{
			UserID:      r.FriendUserID,
			RankEnabled: r.RankEnabled == 1,
			TotalScore:  total,
			WinRate:     0,
			IsBlocked:   blk != nil,
		}
		if denom := win + lose; denom > 0 {
			item.WinRate = float64(win) / float64(denom)
		}
		if b, ok := briefByID[r.FriendUserID]; ok {
			item.Nickname = b.Nickname
			item.Avatar = b.Avatar
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalScore != items[j].TotalScore {
			return items[i].TotalScore > items[j].TotalScore
		}
		return items[i].UserID < items[j].UserID
	})
	return items, nil
}

// Add 加好友：参数 → 对方 Brief 存在（Status 任值均可；校验路径失败即上抛，不同于 List
// 降级）→ 已是好友 30109 → 双向拉黑拦截 30108 → 双向各写一行单事务。
// 存在性在拉黑之前（给定用例注释钉死：关系已存在仍报 30109）。
func (s *friendService) Add(ctx context.Context, userID, friendID uint64) error {
	if err := validatePair(userID, friendID); err != nil {
		return err
	}
	// 对方 Brief 存在性校验（只查对方，不查自己）。micro 模式 briefs 未装配 → 降级跳过。
	if s.briefs != nil {
		bs, err := s.briefs.GetBriefs(ctx, []uint64{friendID})
		if err != nil {
			return err
		}
		found := false
		for _, b := range bs {
			if b.ID == friendID {
				found = true
				break
			}
		}
		if !found {
			return errs.New(errs.CodeUserNotFound, nil)
		}
	}
	// 已是好友 → 30109。
	if existing, err := s.friend.Get(ctx, userID, friendID); err != nil {
		return err
	} else if existing != nil {
		return errs.New(errs.CodeScorecardFriendTaken, nil)
	}
	// 拉黑双向拦截（任一方向命中即 30108）。
	blocked, err := s.black.Exists(ctx, userID, friendID)
	if err != nil {
		return err
	}
	if blocked {
		return errs.New(errs.CodeScorecardBlacklisted, nil)
	}
	// 双向各写一行，单事务（我加你 + 你加我，rank_enabled=1）：tx 内用 tx 绑定 repo
	// （T7 契约）；并发下唯一键撞车映射 30109（关系已存在，含反向行残留场景）。
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txFriend := repository.NewFriendRepository(tx)
		if err := txFriend.Create(ctx, &model.Friend{
			UserID: userID, FriendUserID: friendID, RankEnabled: 1,
		}); err != nil {
			return err
		}
		return txFriend.Create(ctx, &model.Friend{
			UserID: friendID, FriendUserID: userID, RankEnabled: 1,
		})
	})
	if isUniqueViolation(err) {
		return errs.New(errs.CodeScorecardFriendTaken, err)
	}
	return err
}

// Remove 删好友：双向删行单事务（与 Add 的双向插入对称；只删一行会留反向残留，
// 下次 Add 撞唯一键）。DELETE 影响 0 行也返回 nil → 幂等。
func (s *friendService) Remove(ctx context.Context, userID, friendID uint64) error {
	if err := validatePair(userID, friendID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txFriend := repository.NewFriendRepository(tx)
		if err := txFriend.Delete(ctx, userID, friendID); err != nil {
			return err
		}
		return txFriend.Delete(ctx, friendID, userID)
	})
}

// SetBlack 拉黑/解除：blocked=true 幂等 upsert（已存在直接成功；并发撞唯一键也算成功）；
// false 删我方行（不存在也是成功）。判定/进房拦截复用 black.Exists 双向查询，此处只管写入。
func (s *friendService) SetBlack(ctx context.Context, userID, blockedID uint64, blocked bool) error {
	if err := validatePair(userID, blockedID); err != nil {
		return err
	}
	if !blocked {
		return s.black.Delete(ctx, userID, blockedID)
	}
	row, err := s.black.Get(ctx, userID, blockedID)
	if err != nil {
		return err
	}
	if row != nil {
		return nil // 已拉黑 → 幂等成功
	}
	if err := s.black.Create(ctx, &model.FriendBlack{
		UserID: userID, BlockedUserID: blockedID,
	}); err != nil {
		if isUniqueViolation(err) {
			return nil // 并发双拉黑：行已存在即成功
		}
		return err
	}
	return nil
}

// SetRank 排名参与开关：只更新我方 (userID → friendID) 单行 rank_enabled。
// 行不存在时空更新（0 rows 影响），无独立错误码——与 Remove 的 DELETE 幂等语义一致。
func (s *friendService) SetRank(ctx context.Context, userID, friendID uint64, enabled bool) error {
	if err := validatePair(userID, friendID); err != nil {
		return err
	}
	v := 0
	if enabled {
		v = 1
	}
	return s.friend.SetRank(ctx, userID, friendID, v)
}

// AdminList 监管端好友关系分页（userID 维度不过滤：全租户关系行，tenant plugin 隔离）。
func (s *friendService) AdminList(ctx context.Context, page, size int) ([]*model.Friend, int64, error) {
	return s.friend.ListAdmin(ctx, page, size)
}
