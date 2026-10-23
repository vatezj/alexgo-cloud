package service

import (
	"context"
	"errors"
	"sort"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

// Add 好友失败的两个可映射 400 裸 error（controller 直接翻 {"error":...}）。
var (
	ErrSelfFriend     = errors.New("self_friend")
	ErrTargetNotFound = errors.New("target_not_found")
)

// FriendSource 好友源抽象（spec §4.6 微信关系链留位）：
// 一期 StoredFriendSource（库内关系），二期可插 WeChatRelationSource 不动 handler。
type FriendSource interface {
	// FriendsOf 目标 openid 的好友 openid 列表；自己无档案 → nil。
	FriendsOf(ctx context.Context, openid string) ([]string, error)
}

// StoredFriendSource 库内好友关系实现。
type StoredFriendSource struct {
	friends repository.FriendRepository
	players repository.PlayerRepository
}

func NewStoredFriendSource(friends repository.FriendRepository, players repository.PlayerRepository) *StoredFriendSource {
	return &StoredFriendSource{friends: friends, players: players}
}

func (s *StoredFriendSource) FriendsOf(ctx context.Context, openid string) ([]string, error) {
	me, err := s.players.GetByOpenID(ctx, openid)
	if err != nil {
		return nil, err
	}
	if me == nil {
		return nil, nil
	}
	ids, err := s.friends.ListFriendIDs(ctx, me.ID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		p, err := s.players.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, p.OpenID)
		}
	}
	return out, nil
}

// RankRow 好友榜行（契约 {"nick","dex"}）。
type RankRow struct {
	Nick string `json:"nick"`
	Dex  int64  `json:"dex"`
}

// FriendService 好友写入口 + 双口径好友榜（spec §4.6）。
type FriendService struct {
	source  FriendSource
	players repository.PlayerRepository
	friends repository.FriendRepository
	dex     repository.DexRepository
}

func NewFriendService(
	source FriendSource,
	players repository.PlayerRepository,
	friends repository.FriendRepository,
	dex repository.DexRepository,
) *FriendService {
	return &FriendService{source: source, players: players, friends: friends, dex: dex}
}

// Add 加好友：自己/不存在 → 哨兵 error；幂等（已存在 OK）。
func (s *FriendService) Add(ctx context.Context, player *model.Player, targetOpenID string) error {
	if targetOpenID == player.OpenID {
		return ErrSelfFriend
	}
	target, err := s.players.GetByOpenID(ctx, targetOpenID)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrTargetNotFound
	}
	return s.friends.AddPair(ctx, player.ID, target.ID)
}

// rankEntry 榜单中间行（带 player_id：admin 次位查询复用同一双口径逻辑）。
type rankEntry struct {
	PlayerID uint64
	Nick     string
	Dex      int64
}

// rankEntries 双口径榜单：好友源非空 → 好友 ∪ 自己；空 → 同租户全体；
// dex 降序、nick 升序决胜（Rank 与 admin /players/:id/rank 共用）。
func (s *FriendService) rankEntries(ctx context.Context, player *model.Player) ([]rankEntry, error) {
	friendOpenIDs, err := s.source.FriendsOf(ctx, player.OpenID)
	if err != nil {
		return nil, err
	}

	var set []*model.Player
	if len(friendOpenIDs) == 0 {
		set, err = s.players.ListAll(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		set = append(set, player)
		for _, oid := range friendOpenIDs {
			p, err := s.players.GetByOpenID(ctx, oid)
			if err != nil {
				return nil, err
			}
			if p != nil && p.ID != player.ID {
				set = append(set, p)
			}
		}
	}

	ids := make([]uint64, 0, len(set))
	for _, p := range set {
		ids = append(ids, p.ID)
	}
	counts, err := s.dex.CountMapByPlayers(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]rankEntry, 0, len(set))
	for _, p := range set {
		rows = append(rows, rankEntry{PlayerID: p.ID, Nick: p.Nick, Dex: counts[p.ID]})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Dex != rows[j].Dex {
			return rows[i].Dex > rows[j].Dex
		}
		return rows[i].Nick < rows[j].Nick
	})
	return rows, nil
}

// Rank 好友榜契约行（rankEntries 的投影）。
func (s *FriendService) Rank(ctx context.Context, player *model.Player) ([]RankRow, error) {
	entries, err := s.rankEntries(ctx, player)
	if err != nil {
		return nil, err
	}
	rows := make([]RankRow, len(entries))
	for i, e := range entries {
		rows[i] = RankRow{Nick: e.Nick, Dex: e.Dex}
	}
	return rows, nil
}
