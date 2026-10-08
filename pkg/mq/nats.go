package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
)

// NATSBroker 是基于 NATS JetStream 的 Broker 实现。
//
// 设计定位：
// - 用于 Outbox Relay 的“可靠发布”通道（支持 ack、stream 持久化等）。
// - 该 Broker 仅负责 Publish；消费者侧（订阅、重试、DLQ）按业务扩展实现。
//
// 配置项（见 config.yaml / helm values）：
// - mq.nats.enabled: 是否启用
// - mq.nats.url: nats 服务地址（默认 nats://127.0.0.1:4222）
// - mq.nats.stream: JetStream stream 名
// - mq.nats.subject_prefix: stream subjects（如 alexgo.>）
type NATSBroker struct {
	js     nats.JetStreamContext
	stream string
}

// NATSParams 注入 Lifecycle 与配置，保证连接在进程退出时 Drain/Close。
type NATSParams struct {
	fx.In

	LC  fx.Lifecycle
	Cfg *config.Config
}

// NewNATSBroker 在 mq.nats.enabled=true 时创建 NATS 连接并初始化 JetStream stream。
//
// 返回值约定：
// - 未启用时返回 (nil, nil)，上层应把 nil broker 当作“禁用 MQ”处理。
func NewNATSBroker(p NATSParams) (Broker, error) {
	if p.Cfg == nil || !p.Cfg.MQ.NATS.Enabled {
		return nil, nil
	}
	url := p.Cfg.MQ.NATS.URL
	if url == "" {
		url = nats.DefaultURL
	}

	// Connect：建议增加重连策略、鉴权、TLS 等（根据生产环境需要）。
	nc, err := nats.Connect(url,
		nats.Timeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	p.LC.Append(fx.Hook{
		// OnStop：优雅 Drain，尽量把未发送完的消息 flush 出去。
		OnStop: func(ctx context.Context) error {
			nc.Drain()
			nc.Close()
			return nil
		},
	})

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	stream := p.Cfg.MQ.NATS.Stream
	if stream == "" {
		stream = "ALEXGO"
	}
	subjectPrefix := p.Cfg.MQ.NATS.SubjectPrefix
	if subjectPrefix == "" {
		subjectPrefix = "alexgo.>"
	}

	// AddStream：如果 stream 已存在，StreamInfo 可用来确认，无需报错退出。
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{subjectPrefix},
	})
	if err != nil {
		if _, ierr := js.StreamInfo(stream); ierr != nil {
			return nil, fmt.Errorf("create stream failed: %w", err)
		}
	}

	return &NATSBroker{js: js, stream: stream}, nil
}

// Publish 发布消息到 JetStream。
//
// subject 建议遵循事件命名规范，例如：
// - order.created
// - user.updated
//
// 与 stream.subject_prefix 的匹配关系需保证消息会进入目标 stream。
func (b *NATSBroker) Publish(ctx context.Context, subject string, payload []byte) error {
	if b == nil || b.js == nil {
		return fmt.Errorf("nats broker disabled")
	}
	_, err := b.js.Publish(subject, payload, nats.Context(ctx))
	return err
}
