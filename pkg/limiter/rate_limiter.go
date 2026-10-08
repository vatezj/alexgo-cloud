package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter 是基于 Redis 的分布式限流器（Token Bucket）。
//
// 为什么用 Redis：
// - 多实例部署时，限流状态需要跨进程共享；本地内存限流无法做到全局一致。
//
// 为什么用 Lua：
// - Token Bucket 需要“读-算-写”原子性，否则并发下会放大/缩小限流效果。
//
// 注意：
// - 该实现使用“每秒补充 rate 个 token，上限 burst”。
// - 当 Redis 不可用/未启用时，Allow 默认放行（fail-open），避免把 Redis 故障扩大为业务不可用。
type RateLimiter struct {
	redis *redis.Client
}

// NewRateLimiter 构造限流器。
// rdb 为 nil 表示 Redis 未启用，此时 Allow 会默认放行。
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{redis: rdb}
}

// Allow 判断某个 key 在当前时间窗口是否允许通过。
//
// key：通常建议包含 tenant / ip / route 等维度，例如：
// - <tenant>:<client_ip>:<route>
//
// rate：每秒补充 token 数（近似 QPS）
// burst：桶容量上限（允许短时间突刺）
func (l *RateLimiter) Allow(ctx context.Context, key string, rate int, burst int) bool {
	if l == nil || l.redis == nil {
		return true
	}
	if rate <= 0 || burst <= 0 {
		return true
	}

	// 使用秒级时间戳：足够满足大多数 API 场景；如需更精细可用毫秒并相应调整脚本。
	now := time.Now().Unix()
	ttl := int64(60)
	if rate > 0 {
		ttl = int64(burst/rate) + 60
		if ttl < 60 {
			ttl = 60
		}
	}

	script := redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
  tokens = burst
  ts = now
end

local delta = math.max(0, now - ts)
local filled = math.min(burst, tokens + (delta * rate))
local allowed = 0

if filled >= 1 then
  filled = filled - 1
  allowed = 1
end

redis.call("HMSET", key, "tokens", filled, "ts", now)
redis.call("EXPIRE", key, ttl)
return allowed
`)

	// Eval：原子执行 token bucket 更新与判断。
	res, err := script.Run(ctx, l.redis, []string{"rate:" + key}, now, rate, burst, ttl).Int()
	if err != nil {
		// Redis 故障时 fail-open，避免限流组件成为单点故障。
		return true
	}
	return res == 1
}
