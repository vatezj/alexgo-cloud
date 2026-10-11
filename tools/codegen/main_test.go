// tools/codegen/main_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"alexGo-cloud/pkg/codegen/model"
)

func TestWriteFiles_NoOverwrite(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "modules/order/model/order.go")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []model.GeneratedFile{
		{Path: "modules/order/model/order.go", Content: []byte("NEW")},
		{Path: "modules/order/model/fresh.go", Content: []byte("FRESH")},
	}

	created, skipped, err := writeFiles(root, files, false)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != 1 || skipped != 1 {
		t.Fatalf("created=%d skipped=%d, want 1/1", created, skipped)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "ORIGINAL" {
		t.Errorf("existing file overwritten: %q", got)
	}
	fresh, err := os.ReadFile(filepath.Join(root, "modules/order/model/fresh.go"))
	if err != nil || string(fresh) != "FRESH" {
		t.Errorf("fresh file = %q, err=%v", fresh, err)
	}
}

func TestWriteFiles_ForceOverwrites(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "a.go")
	if err := os.WriteFile(existing, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, skipped, err := writeFiles(root, []model.GeneratedFile{{Path: "a.go", Content: []byte("NEW")}}, true)
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}
	if created != 1 || skipped != 0 {
		t.Fatalf("created=%d skipped=%d, want 1/0", created, skipped)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "NEW" {
		t.Errorf("content = %q, want NEW", got)
	}
}

func TestSchemaFromDSN(t *testing.T) {
	cases := []struct {
		dsn  string
		want string
	}{
		{"user:pass@tcp(127.0.0.1:3306)/alexgo?charset=utf8mb4", "alexgo"},
		{"user:pass@tcp(127.0.0.1:3306)/alexgo", "alexgo"},
		// 缺陷①：unix socket 无库名——手写解析返回 "mysqld.sock)"，ParseDSN 返回 ""
		{"user:pass@unix(/var/run/mysqld/mysqld.sock)", ""},
		{"user:pass@unix(/var/run/mysqld/mysqld.sock)/alexgo", "alexgo"},
		// 缺陷②：密码含 @ 的 case 手写 LastIndex("/") 恰好不炸（密码段无 /），
		// 保留为特征化。query 含未转义 /（dir=/x）按驱动转义规则是非法 DSN——
		// ParseDSN 报错，契约返回 ""（手写版会错截成 "x"/"db1"）。
		{"user:p@ss@word@tcp(h:3306)/db1", "db1"},
		{"user:pass@tcp(h:3306)/db1?dir=/x", ""},
	}
	for _, tc := range cases {
		if got := schemaFromDSN(tc.dsn); got != tc.want {
			t.Errorf("schemaFromDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
		}
	}
}
