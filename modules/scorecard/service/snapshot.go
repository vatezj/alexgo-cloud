package service

import "time"

// RoomMember 快照成员行。昵称头像由 member BriefProvider 批量补齐，拉取失败降级留空。
type RoomMember struct {
	UserID       uint64 `json:"user_id"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	CurScore     int    `json:"cur_score"`
	CumLoseScore int    `json:"cum_lose_score"`
	IsCreator    bool   `json:"is_creator"`
}

// RoomLogItem 快照内嵌流水条目（RecentLogs 最近 20 条，spec §4.1 快照含最近日志）。
type RoomLogItem struct {
	ID              uint64    `json:"id"`
	FromUserID      uint64    `json:"from_user_id"`
	ToUserID        uint64    `json:"to_user_id"`
	Score           int       `json:"score"`
	Tea             int       `json:"tea"`
	Type            int       `json:"type"`
	ClientRequestID string    `json:"client_request_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// RoomSnapshot 房间全量快照：version/成员/配置/茶饭池/最近日志。客户端以服务端为准，
// 按 version 去重丢弃过期推送（spec §4.1/弱网策略）。
type RoomSnapshot struct {
	ID                  uint64        `json:"id"`
	RoomCode            string        `json:"room_code"`
	CreatorID           uint64        `json:"creator_id"`
	Status              int           `json:"status"`
	Version             int           `json:"version"`
	ExpireTime          time.Time     `json:"expire_time"`
	TeaTargetAmount     int           `json:"tea_target_amount"`
	TeaPercent          float64       `json:"tea_percent"`
	TeaPoolTotal        int           `json:"tea_pool_total"`
	TeaAutoEnabled      bool          `json:"tea_auto_enabled"`
	MaxLoseScore        int           `json:"max_lose_score"`
	RoundNo             int           `json:"round_no"`
	PendingApplyRoundNo int           `json:"pending_apply_round_no"`
	Members             []RoomMember  `json:"members"`
	RecentLogs          []RoomLogItem `json:"recent_logs"` // 最近 20 条流水（spec §4.1 快照含最近日志）
}

// CreateRoomReq 开房请求。RoomCode 可空 = 服务端生成 4 位房号。
type CreateRoomReq struct {
	RoomCode        string  `json:"room_code"` // 可空=服务端生成
	TeaTargetAmount int     `json:"tea_target_amount"`
	TeaPercent      float64 `json:"tea_percent"`
	TeaAutoEnabled  bool    `json:"tea_auto_enabled"`
	MaxLoseScore    int     `json:"max_lose_score"`
}
