package service

import "context"

// UserBrief 会员公开摘要：跨模块展示昵称/头像的最小面（不含手机号等敏感列）。
type UserBrief struct {
	ID       uint64 `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   int    `json:"status"`
}

// BriefProvider member 对外的窄只读接口（scorecard 等业务模块消费；fx 映射见 module.go）。
type BriefProvider interface {
	GetBriefs(ctx context.Context, ids []uint64) ([]UserBrief, error)
}
