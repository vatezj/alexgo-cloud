package service

import (
	"sync"
	"time"
)

// TTLCache 进程内存 TTL 缓存，时钟可注入（生产传 time.Now，测试传假钟）。
// 仅进程内有效，不跨实例共享。
type TTLCache struct {
	now func() time.Time
	mu  sync.Mutex
	m   map[string]entry
}

type entry struct {
	val any
	exp time.Time
}

// NewTTLCache 构造缓存；now 为时钟源，nil 时回退 time.Now。
func NewTTLCache(now func() time.Time) *TTLCache {
	if now == nil {
		now = time.Now
	}
	return &TTLCache{
		now: now,
		m:   make(map[string]entry),
	}
}

// Get 返回未过期的值；过期项返回 (nil,false) 并删除该条目。
func (c *TTLCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(e.exp) { // now >= exp → 过期
		delete(c.m, key)
		return nil, false
	}
	return e.val, true
}

// Set 写入 key，自当前时钟起 ttl 后过期；同 key 覆盖旧值与旧过期时间。
func (c *TTLCache) Set(key string, v any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = entry{val: v, exp: c.now().Add(ttl)}
}
