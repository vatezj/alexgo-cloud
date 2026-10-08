package model

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID          uint64          `gorm:"primaryKey" json:"id"`
	AggregateID string          `json:"aggregate_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at"`
	TenantID    uint64          `json:"tenant_id"`
}
