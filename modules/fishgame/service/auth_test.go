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
	svc := NewAuthService(cli, repository.NewPlayerRepository(db), repository.NewSessionRepository(db))

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
	svc := NewAuthService(cli, repository.NewPlayerRepository(db), repository.NewSessionRepository(db))
	if _, err := svc.Login(t.Context(), "bad"); err == nil {
		t.Fatal("wechat 失败应返回 error")
	}
}
