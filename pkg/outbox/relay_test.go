package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	systemmodel "alexGo-cloud/modules/system/model"
	"alexGo-cloud/pkg/mq"
)

type fakeBroker struct {
	err     error
	publish []string // 记录收到的 subject
}

func (f *fakeBroker) Publish(_ context.Context, subject string, _ []byte) error {
	if f.err != nil {
		return f.err
	}
	f.publish = append(f.publish, subject)
	return nil
}

var _ mq.Broker = (*fakeBroker)(nil)

func events(ids ...uint64) []systemmodel.OutboxEvent {
	var out []systemmodel.OutboxEvent
	for _, id := range ids {
		out = append(out, systemmodel.OutboxEvent{
			ID:        id,
			EventType: "order.created",
			Payload:   json.RawMessage(`{"id":1}`),
			Status:    "pending",
		})
	}
	return out
}

// 全部发布成功 → 每条回调 published。
func TestPublishEvents_AllSuccess(t *testing.T) {
	b := &fakeBroker{}
	type marked struct {
		id     uint64
		status string
	}
	var got []marked
	err := publishEvents(context.Background(), b, events(1, 2, 3),
		func(id uint64, status string, _ *time.Time) error {
			got = append(got, marked{id, status})
			return nil
		})
	if err != nil {
		t.Fatalf("publishEvents() error = %v", err)
	}
	if len(b.publish) != 3 {
		t.Errorf("published %d, want 3", len(b.publish))
	}
	if len(got) != 3 {
		t.Fatalf("mark called %d times, want 3", len(got))
	}
	for i, g := range got {
		if g.status != "published" {
			t.Errorf("mark[%d] status = %q, want published", i, g.status)
		}
	}
}

// Broker 发布失败 → 对应事件回调 failed，且不影响后续事件。
func TestPublishEvents_PublishFailureMarksFailed(t *testing.T) {
	b := &fakeBroker{err: errors.New("nats down")}
	statuses := map[uint64]string{}
	err := publishEvents(context.Background(), b, events(1, 2),
		func(id uint64, status string, _ *time.Time) error {
			statuses[id] = status
			return nil
		})
	if err != nil {
		t.Fatalf("publishEvents() error = %v", err)
	}
	for _, id := range []uint64{1, 2} {
		if statuses[id] != "failed" {
			t.Errorf("event %d status = %q, want failed", id, statuses[id])
		}
	}
}

// 发布成功但状态回写失败 → 返回错误（让调用方感知，事务回滚语义由 processPending 决定）。
func TestPublishEvents_MarkErrorPropagates(t *testing.T) {
	b := &fakeBroker{}
	err := publishEvents(context.Background(), b, events(1),
		func(uint64, string, *time.Time) error { return errors.New("db down") })
	if err == nil {
		t.Error("mark error must propagate")
	}
}

// nil 守卫：processPending 在 db/broker 为 nil 时必须 no-op。
func TestProcessPending_NilGuards(t *testing.T) {
	var nilRelay *Relay
	if err := nilRelay.processPending(context.Background(), 10); err != nil {
		t.Errorf("nil relay err = %v, want nil", err)
	}
	r := NewRelay(nil, nil, nil)
	if err := r.processPending(context.Background(), 10); err != nil {
		t.Errorf("nil db/broker err = %v, want nil", err)
	}
}
