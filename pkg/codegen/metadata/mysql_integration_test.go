//go:build integration

package metadata_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"

	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/model"
)

// TestMySQLReader_RealDB 需要真实 MySQL：DB_DSN 未设置或连不上时跳过。
func TestMySQLReader_RealDB(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN not set; skip integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("mysql unreachable: %v", err)
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		t.Fatalf("cannot parse db name from dsn: %v", err)
	}

	reader := metadata.NewMySQLReader(db, cfg.DBName)

	tables, err := reader.ListTables(ctx)
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("ListTables returned 0 tables")
	}

	meta, err := reader.ReadTable(ctx, tables[0])
	if err != nil {
		t.Fatalf("ReadTable(%s): %v", tables[0], err)
	}
	if meta.Name != tables[0] || len(meta.Columns) == 0 {
		t.Fatalf("meta = %+v, want table %s with columns", meta, tables[0])
	}

	if _, err := reader.ReadTable(ctx, "definitely_not_a_table_xyz"); !errors.Is(err, model.ErrTableNotFound) {
		t.Fatalf("err = %v, want ErrTableNotFound", err)
	}
}
