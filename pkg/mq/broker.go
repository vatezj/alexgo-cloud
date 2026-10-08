package mq

import "context"

// Broker 抽象消息发布接口。
//
// 设计目的：
// - 业务层只依赖 Broker，不关心底层是 NATS/Kafka/RabbitMQ 还是 mock。
// - 便于在“单体模式 → 微服务模式”演进时替换实现、做条件注入。
//
// 语义约定：
// - subject：事件主题/路由键（例如 order.created）
// - payload：序列化后的事件数据（通常为 JSON 或 protobuf bytes）
type Broker interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}
