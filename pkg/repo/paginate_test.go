package repo

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type pageRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func (pageRow) TableName() string { return "page_rows" }

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&pageRow{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 25; i++ {
		if err := db.Create(&pageRow{Name: "n"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestPaginate(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(),
		db.Where("id > 0").Order("id asc"), 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 {
		t.Fatalf("total = %d, want 25", total)
	}
	if len(items) != 10 {
		t.Fatalf("len = %d, want 10", len(items))
	}
	if items[0].ID != 11 || items[9].ID != 20 {
		t.Fatalf("wrong page: first=%d last=%d", items[0].ID, items[9].ID)
	}
}

func TestPaginateLastPage(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(),
		db.Order("id asc"), 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 || len(items) != 5 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
}

func TestPaginateZeroArgsDefensive(t *testing.T) {
	db := setupDB(t)
	items, total, err := Paginate[pageRow](context.Background(), db, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 || len(items) != 10 { // 兜底 1/10
		t.Fatalf("total=%d len=%d", total, len(items))
	}
}
