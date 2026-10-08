package model

import "time"

type LoginLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `json:"tenant_id"`
	Username  string    `json:"username"`
	UserID    uint64    `json:"user_id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Success   int       `json:"success"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func (LoginLog) TableName() string { return "login_logs" }

type OperateLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `json:"tenant_id"`
	UserID    uint64    `json:"user_id"`
	Username  string    `json:"username"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	LatencyMs int64     `json:"latency_ms"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
}

func (OperateLog) TableName() string { return "operate_logs" }

