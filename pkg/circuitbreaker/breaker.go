package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen 表示熔断器处于 Open 状态，当前请求应当快速失败（fast-fail）。
var ErrOpen = errors.New("circuit breaker open")

// State 表示熔断器状态机。
//
// - Closed：正常放行，统计连续失败次数
// - Open：直接拒绝请求，等待 openFor 时间窗口结束
// - HalfOpen：放行“少量探测请求”，成功则恢复 Closed，失败则回到 Open
type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

// CircuitBreaker 是一个轻量的熔断器实现（无外部依赖，适合做入口级保护）。
//
// 设计取舍：
// - 以“连续失败次数 >= threshold”作为打开条件（简单、可解释）。
// - HalfOpen 仅允许单个探测请求（halfInUse），避免恢复期被并发流量瞬间压垮。
//
// 注意：
// - 此实现更偏“入口防护”。如果用于下游调用（DB/Redis/HTTP client），通常建议为每个下游/每个资源单独实例化。
type CircuitBreaker struct {
	mu        sync.Mutex
	state     State
	failures  int
	threshold int
	openFor   time.Duration
	openedAt  time.Time
	halfInUse bool
}

// NewCircuitBreaker 创建一个熔断器。
// - threshold：连续失败次数达到该值进入 Open
// - openFor：Open 状态保持时间（到期进入 HalfOpen）
func NewCircuitBreaker(threshold int, openFor time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 10
	}
	if openFor <= 0 {
		openFor = 30 * time.Second
	}
	return &CircuitBreaker{state: Closed, threshold: threshold, openFor: openFor}
}

// Do 执行受熔断保护的函数。
//
// 典型用法：
// err := cb.Do(func() error { return callDownstream() })
//
// 在 Gin 中间件场景：
// - 先执行 c.Next()
// - 如果响应码 >= 500，则视为一次失败
func (cb *CircuitBreaker) Do(fn func() error) error {
	if cb == nil {
		return fn()
	}

	if !cb.allow() {
		return ErrOpen
	}

	err := fn()
	cb.after(err == nil)
	return err
}

// allow 在进入 fn 前判断是否允许执行，并在 HalfOpen 状态设置“探测占用”标记。
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		return true
	case Open:
		if time.Since(cb.openedAt) >= cb.openFor {
			cb.state = HalfOpen
			cb.halfInUse = true
			return true
		}
		return false
	case HalfOpen:
		if cb.halfInUse {
			return false
		}
		cb.halfInUse = true
		return true
	default:
		return true
	}
}

// after 在 fn 执行后根据 success 更新状态机。
func (cb *CircuitBreaker) after(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == HalfOpen {
		cb.halfInUse = false
		if success {
			cb.failures = 0
			cb.state = Closed
			return
		}
		cb.state = Open
		cb.openedAt = time.Now()
		return
	}

	if success {
		cb.failures = 0
		cb.state = Closed
		return
	}

	cb.failures++
	if cb.failures >= cb.threshold {
		cb.state = Open
		cb.openedAt = time.Now()
	}
}
