package social

import "testing"

func TestAdapterRegistry(t *testing.T) {
	r := NewAdapterRegistry() // 注意：Task 5 才有 wechat_web——本步先只 Register(NewWechatMP())
	if !r.Exists(PlatformWechatMP) {
		t.Error("registry 缺 wechat_mp")
	}
	if _, ok := r.Get("douyin"); ok {
		t.Error("未知平台应 miss")
	}
}

func TestAdapterRegistry_DupPanics(t *testing.T) {
	r := &AdapterRegistry{byPlatform: make(map[string]PlatformAdapter)}
	r.Register(NewWechatMP())
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("重复注册应 panic")
		} else if rec != "social: adapter already registered: "+PlatformWechatMP {
			t.Errorf("panic 值 = %v", rec)
		}
	}()
	r.Register(NewWechatMP())
}
