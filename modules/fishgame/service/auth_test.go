package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Player{}, &model.Session{}, &model.Fish{}, &model.Dex{},
		&model.TrackEvent{}, &model.Sign{}, &model.Friend{}, &model.Config{}, &model.Share{},
	); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAuthService_Login_Success(t *testing.T) {
	db := newTestDB(t)
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"openid":"openid-ok"}`))
	})
	svc := NewAuthService(cli, repository.NewPlayerRepository(db), repository.NewSessionRepository(db), nil, "")

	token, err := svc.Login(t.Context(), "code-ok")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("token 不应为空")
	}

	// player 已落行
	var p model.Player
	if err := db.Where("openid = ?", "openid-ok").First(&p).Error; err != nil {
		t.Fatalf("player 未落行: %v", err)
	}
	if p.Nick == "" {
		t.Fatal("nick 占位不应为空")
	}

	// session 已落行且未过期
	var s model.Session
	if err := db.Where("token = ?", token).First(&s).Error; err != nil {
		t.Fatalf("session 未落行: %v", err)
	}
	if !s.ExpireAt.After(time.Now()) {
		t.Fatalf("expire_at 应在未来: %v", s.ExpireAt)
	}
	if s.PlayerID != p.ID {
		t.Fatalf("session.player_id = %d, player.id = %d", s.PlayerID, p.ID)
	}
}

func TestAuthService_Login_WechatFail(t *testing.T) {
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	})
	db := newTestDB(t)
	svc := NewAuthService(cli, repository.NewPlayerRepository(db), repository.NewSessionRepository(db), nil, "")
	if _, err := svc.Login(t.Context(), "bad"); err == nil {
		t.Fatal("wechat 失败应返回 error")
	}
}

// ---- 测试账号登录（编辑器联调口，名单+密钥双闸） ----

func newTestAuthService(t *testing.T, accounts []string, secret string) (*AuthService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	cli := newTestWechat(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"openid":"unused"}`))
	})
	return NewAuthService(cli, repository.NewPlayerRepository(db), repository.NewSessionRepository(db), accounts, secret), db
}

func TestAuthService_TestAccounts_List(t *testing.T) {
	svc, _ := newTestAuthService(t, []string{"alice", "bob"}, "s3cret")
	got := svc.TestAccounts()
	if len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("TestAccounts() = %v", got)
	}
}

func TestAuthService_TestLogin_Success_AutoCreate(t *testing.T) {
	svc, db := newTestAuthService(t, []string{"alice", "bob"}, "s3cret")

	token, err := svc.TestLogin(t.Context(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("名单内+密钥对应通过: %v", err)
	}
	if token == "" {
		t.Fatal("token 不应为空")
	}
	// 合成 openid 落行
	var p model.Player
	if err := db.Where("openid = ?", "test_alice").First(&p).Error; err != nil {
		t.Fatalf("test_alice 未自动创建: %v", err)
	}
	// 二次登录复用同一 player（不裂行）
	token2, err := svc.TestLogin(t.Context(), "alice", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&model.Player{}).Where("openid = ?", "test_alice").Count(&n)
	if n != 1 {
		t.Fatalf("二次登录应复用 player，行数 = %d", n)
	}
	if token2 == "" || token2 == token {
		t.Fatal("每次应签发新 token")
	}
	// session 归属正确
	var s model.Session
	if err := db.Where("token = ?", token).First(&s).Error; err != nil {
		t.Fatal(err)
	}
	if s.PlayerID != p.ID {
		t.Fatalf("session.player_id = %d, player.id = %d", s.PlayerID, p.ID)
	}
}

func TestAuthService_TestLogin_AccountNotInList(t *testing.T) {
	svc, db := newTestAuthService(t, []string{"alice"}, "s3cret")
	if _, err := svc.TestLogin(t.Context(), "mallory", "s3cret"); err == nil {
		t.Fatal("名单外账号应拒绝")
	}
	var n int64
	db.Model(&model.Player{}).Count(&n)
	if n != 0 {
		t.Fatalf("拒绝时不应建号，行数 = %d", n)
	}
}

func TestAuthService_TestLogin_WrongSecret(t *testing.T) {
	svc, db := newTestAuthService(t, []string{"alice"}, "s3cret")
	if _, err := svc.TestLogin(t.Context(), "alice", "wrong"); err == nil {
		t.Fatal("密钥错误应拒绝")
	}
	if _, err := svc.TestLogin(t.Context(), "alice", ""); err == nil {
		t.Fatal("密钥为空应拒绝（配置了密钥时）")
	}
	var n int64
	db.Model(&model.Player{}).Count(&n)
	if n != 0 {
		t.Fatalf("拒绝时不应建号，行数 = %d", n)
	}
}

func TestAuthService_TestLogin_EmptyAccount(t *testing.T) {
	svc, _ := newTestAuthService(t, []string{"alice"}, "s3cret")
	if _, err := svc.TestLogin(t.Context(), "", "s3cret"); err == nil {
		t.Fatal("空账号应拒绝")
	}
}
