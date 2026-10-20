package model

import "time"

// RoomLog 流水。Type: 1 记分 2 茶饭 3 进房 4 退房 5 结算 6 上限提醒 7 配置变更 8 状态变更 9 转让房主。
// ClientRequestID 为支出幂等键（type=1 时必填且房间内唯一）；Extra 存触发累计值等扩展 JSON 文本。
type RoomLog struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID        uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	RoomID          uint64    `gorm:"column:room_id;index:idx_room_log_room_created,priority:1" json:"room_id"`
	FromUserID      uint64    `gorm:"column:from_user_id" json:"from_user_id"`
	ToUserID        uint64    `gorm:"column:to_user_id" json:"to_user_id"`
	Score           int       `gorm:"column:score" json:"score"`
	Tea             int       `gorm:"column:tea" json:"tea"`
	Type            int       `gorm:"column:type" json:"type"`
	ClientRequestID string    `gorm:"column:client_request_id;size:64" json:"client_request_id"`
	Extra           string    `gorm:"column:extra;type:text" json:"extra"`
	CreatedAt       time.Time `gorm:"column:created_at;index:idx_room_log_room_created,priority:2" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定房间流水表名（单数，与迁移 DDL 一致）。
func (RoomLog) TableName() string { return "room_log" }
