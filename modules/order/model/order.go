package model

import "time"

type Order struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	UserID    uint64    `json:"user_id"`
	ProductID uint64    `json:"product_id"`
	Amount    int64     `json:"amount"`
	Status    int       `json:"status"`
	OrderNo   string    `json:"order_no"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
