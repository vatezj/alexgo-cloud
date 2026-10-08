package model

import "time"

type User struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	PasswordHash string `json:"-"`
	Status    int       `json:"status"`
	TenantID   uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
