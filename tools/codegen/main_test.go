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
