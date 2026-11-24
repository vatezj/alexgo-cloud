package service

import (
	"testing"
	"time"
)

// fakeClock 测试假钟：手动推进，不依赖真实时间。
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func TestTTLCache_HitMissExpiry(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	c := NewTTLCache(clk.Now)

	// Set → Get 命中
	c.Set("k", "v", time.Second)
	got, ok := c.Get("k")
	if !ok {
		t.Fatal("Get(k) after Set: miss, want hit")
	}
	if got != "v" {
		t.Fatalf("Get(k) = %v, want %q", got, "v")
	}

	// 假钟推进 ttl+1ns → miss（且过期项被删除）
	clk.advance(time.Second + time.Nanosecond)
	if v, ok := c.Get("k"); ok {
		t.Fatalf("Get(k) after expiry = (%v, true), want miss", v)
	}
	// 后续 Get 仍 miss
	if v, ok := c.Get("k"); ok {
		t.Fatalf("second Get(k) after expiry = (%v, true), want miss", v)
	}

	// 从未 Set 的 key miss
	if v, ok := c.Get("never-set"); ok {
		t.Fatalf("Get(never-set) = (%v, true), want miss", v)
	}
}
