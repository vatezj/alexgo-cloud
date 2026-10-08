package tenant

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// NewDomainLookup：命中/停用/软删/未命中/空 host/db 各分支。
func TestNewDomainLookup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE tenants (
			id INTEGER PRIMARY KEY,
			name TEXT,
			status INTEGER NOT NULL DEFAULT 1,
			domain TEXT NOT NULL DEFAULT '',
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT INTO tenants (id, name, status, domain, deleted) VALUES
			(1, 'a', 1, 'a.com', 0),
			(2, 'b', 0, 'off.com', 0),
			(3, 'c', 1, 'gone.com', 1),
			(4, 'd', 1, '', 0)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	lookup := NewDomainLookup(db)
	ctx := context.Background()

	cases := []struct {
		host string
		want uint64
	}{
		{"a.com", 1},       // 正常命中
		{"off.com", 0},     // status=0 停用
		{"gone.com", 0},    // deleted=1 软删
		{"missing.com", 0}, // 未命中 → 0,nil（平台租户）
		{"", 0},            // 空 host 短路，不打库
	}
	for _, c := range cases {
		got, err := lookup(ctx, c.host)
		if err != nil {
			t.Fatalf("lookup(%q) err=%v", c.host, err)
		}
		if got != c.want {
			t.Fatalf("lookup(%q) = %d, want %d", c.host, got, c.want)
		}
	}

	// db 为 nil 时不 panic，返回 0。
	var nilDB *gorm.DB
	got, err := NewDomainLookup(nilDB)(ctx, "a.com")
	if err != nil || got != 0 {
		t.Fatalf("nil db lookup = (%d, %v), want (0, nil)", got, err)
	}
}
