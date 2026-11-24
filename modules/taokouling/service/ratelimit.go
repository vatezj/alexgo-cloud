package service

import (
	"sync"
	"time"
)

// RateLimiter 按 IP 固定窗口限流，时钟可注入（生产传 time.Now，测试传假钟）。
// 构造方负责 limit 归一（limit≥1），函数内按 limit≥1 处理。
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time
	mu     sync.Mutex
	m      map[string]*bucket
}

type bucket struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter 构造限流器；now 为时钟源，nil 时回退 time.Now。
func NewRateLimiter(limit int, window time.Duration, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{
		limit:  limit,
		window: window,
		now:    now,
		m:      make(map[string]*bucket),
	}
}

// Allow 判定 ip 是否放行：窗口内 count<limit 则 count++ 并 true；
// 窗口过期则重置 bucket（count=1、新 resetAt）并 true；否则 false。
// 每次调用顺带惰性清扫过期 bucket（单趟 O(n)），限制 map 膨胀。
func (c *RateLimiter) Allow(ip string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()

	// 惰性清扫：删除已过期窗口的 bucket（range 中 delete 安全）。
	for k, b := range c.m {
		if !now.Before(b.resetAt) { // now >= resetAt → 窗口过期
			delete(c.m, k)
		}
	}

	b, ok := c.m[ip]
	if !ok { // 新窗口（含刚被清扫的过期 bucket）
		c.m[ip] = &bucket{count: 1, resetAt: now.Add(c.window)}
		return true
	}
	if b.count < c.limit {
		b.count++
		return true
	}
	return false
}
