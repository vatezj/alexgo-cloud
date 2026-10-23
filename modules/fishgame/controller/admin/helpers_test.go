package admin

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// newAdminTestDB 控制器测试内存库（与 app 层 newAppTestDB 同款）。
func newAdminTestDB(t *testing.T) *gorm.DB {
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
