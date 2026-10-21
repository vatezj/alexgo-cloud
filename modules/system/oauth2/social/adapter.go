package social

import (
	"context"
	"sync"
)

// Identity 平台换取到的第三方身份（openid/unionid）。
type Identity struct {
	OpenID   string
	UnionID  string // 可空
	Nickname string // 可空（微信已匿名化）
	Avatar   string // 可空
}

// PlatformAdapter 第三方登录平台适配器：用授权 code 换取身份。
type PlatformAdapter interface {
	Platform() string
	Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error)
}

// AdapterRegistry 平台适配器注册表（装配期构造，运行期只读；RWMutex 防并发 Register）。
type AdapterRegistry struct {
	mu         sync.RWMutex
	byPlatform map[string]PlatformAdapter
}

// NewAdapterRegistry 构造并预注册微信实现。
// 当前仅 wechat_mp（wechat_web 由 Task 5 落地后在此补齐注册）。
func NewAdapterRegistry() *AdapterRegistry {
	r := &AdapterRegistry{byPlatform: make(map[string]PlatformAdapter)}
	r.Register(NewWechatMP())
	return r
}

// Register 注册适配器；平台重复注册是装配期编程错误，直接 panic。
func (r *AdapterRegistry) Register(a PlatformAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := a.Platform()
	if _, dup := r.byPlatform[p]; dup {
		panic("social: adapter already registered: " + p)
	}
	r.byPlatform[p] = a
}

// Get 按平台取适配器，miss 返回 (nil, false)。
func (r *AdapterRegistry) Get(platform string) (PlatformAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.byPlatform[platform]
	return a, ok
}

// Exists 平台是否已注册。
func (r *AdapterRegistry) Exists(platform string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byPlatform[platform]
	return ok
}
