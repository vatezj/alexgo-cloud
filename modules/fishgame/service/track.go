package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

const (
	trackBatchCap = 200
	trackPropsCap = 4 << 10 // 4KB
)

// TrackEventReq /track 契约单条。
type TrackEventReq struct {
	Name  string         `json:"name"`
	T     int64          `json:"t"`
	Props map[string]any `json:"props"`
}

// TrackService 埋点事实流 + catch→dex 推导（spec §4.4）。
type TrackService struct {
	events repository.TrackEventRepository
	fishes repository.FishRepository
	dex    repository.DexRepository
}

func NewTrackService(
	events repository.TrackEventRepository,
	fishes repository.FishRepository,
	dex repository.DexRepository,
) *TrackService {
	return &TrackService{events: events, fishes: fishes, dex: dex}
}

// Track 批量落库 + catch 推导。单条非法跳过；整批空/全非法仍无错（客户端只认 204）。
func (s *TrackService) Track(ctx context.Context, player *model.Player, reqs []TrackEventReq) error {
	if len(reqs) > trackBatchCap {
		reqs = reqs[:trackBatchCap]
	}
	var rows []*model.TrackEvent
	for _, r := range reqs {
		if r.Name == "" || r.T == 0 {
			continue
		}
		props, err := canonicalJSON(r.Props)
		if err != nil {
			props = "{}"
		}
		if len(props) > trackPropsCap {
			props = props[:trackPropsCap]
		}
		rows = append(rows, &model.TrackEvent{
			TenantID: 0,
			PlayerID: player.ID,
			Name:     r.Name,
			T:        r.T,
			Props:    props,
			DedupKey: dedupKey(player.OpenID, r.Name, r.T, props),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	inserted, err := s.events.BatchInsertIgnoreDup(ctx, rows)
	if err != nil {
		return err
	}
	// 仅首次插入的 catch 推导 dex（防重发膨胀）
	for _, i := range inserted {
		row := rows[i]
		if row.Name != "catch" {
			continue
		}
		var props map[string]any
		if err := json.Unmarshal([]byte(row.Props), &props); err != nil {
			continue
		}
		if props["kind"] != "fish" {
			continue
		}
		fishID, _ := props["fish_id"].(string)
		if fishID == "" {
			continue
		}
		// fish_id 不在图鉴表 → warn 跳过不报错
		f, err := s.fishes.GetByFishID(ctx, fishID)
		if err != nil || f == nil {
			continue
		}
		if _, err := s.dex.UpsertCatch(ctx, player.ID, fishID); err != nil {
			return err
		}
	}
	return nil
}

// dedupKey sha1(openid|name|t|canonical_json)。
func dedupKey(openid, name string, t int64, props string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%d|%s", openid, name, t, props)))
	return hex.EncodeToString(h[:])
}

// canonicalJSON map 键排序后序列化（键序不同仍去重）。
func canonicalJSON(m map[string]any) (string, error) {
	if m == nil {
		return "{}", nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf []byte
	buf = append(buf, '{')
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return "", err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		vb, err := json.Marshal(m[k])
		if err != nil {
			return "", err
		}
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return string(buf), nil
}
