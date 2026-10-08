package limiter_test

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/logging"

	"alexGo-cloud/pkg/limiter"
)

func TestMain(m *testing.M) {
	redis.SetLogger(&logging.VoidLogger{}) // 静默 go-redis 内部日志，保证测试输出纯净
	os.Exit(m.Run())
}

func newTestLimiter(t *testing.T) (*limiter.RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return limiter.NewRateLimiter(rdb), mr
}

// burst 内放行、桶耗尽拒绝（Token Bucket 核心语义）。
func TestAllow_TokenBucketExhaustion(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()

	if !l.Allow(ctx, "k1", 1, 2) {
		t.Fatal("1st request should pass (bucket starts full)")
	}
	if !l.Allow(ctx, "k1", 1, 2) {
		t.Fatal("2nd request should pass (burst=2)")
	}
	if l.Allow(ctx, "k1", 1, 2) {
		t.Error("3rd request should be limited (bucket empty)")
	}
}

// 不同 key 互不影响（限流维度隔离）。
func TestAllow_KeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()

	_ = l.Allow(ctx, "a", 1, 1)
	_ = l.Allow(ctx, "a", 1, 1) // a 已耗尽
	if !l.Allow(ctx, "b", 1, 1) {
		t.Error("key b should have its own bucket")
	}
}

// fail-open：未配置 Redis / Redis 宕机 / 非法参数时一律放行。
func TestAllow_FailOpen(t *testing.T) {
	ctx := context.Background()

	var nilLimiter *limiter.RateLimiter
	if !nilLimiter.Allow(ctx, "k", 10, 10) {
		t.Error("nil limiter must allow (fail-open)")
	}

	if !limiter.NewRateLimiter(nil).Allow(ctx, "k", 10, 10) {
		t.Error("nil redis must allow (fail-open)")
	}

	l, mr := newTestLimiter(t)
	if !l.Allow(ctx, "k", 0, 0) {
		t.Error("rate/burst <= 0 must allow")
	}

	mr.Close() // 模拟 Redis 宕机
	if !l.Allow(ctx, "k", 10, 10) {
		t.Error("redis outage must allow (fail-open)")
	}
}
