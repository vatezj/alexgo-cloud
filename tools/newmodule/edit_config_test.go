package newmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditConfigYAML(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "alexgo-server/configs/config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := editConfigYAML(src, "demo")
	if err != nil {
		t.Fatalf("editConfigYAML() = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "demo: true") {
		t.Error("missing demo: true")
	}
	// 既有行与注释保留（抽查）
	for _, keep := range []string{"system: true", "order: true", "# 盘根"} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost content %q", keep)
		}
	}
	// 重名拒绝
	if _, err := editConfigYAML(out, "demo"); err == nil {
		t.Error("duplicate insert should fail")
	}
}

func TestEditConfigYAML_MissingModules(t *testing.T) {
	if _, err := editConfigYAML([]byte("system:\n  dsn: x\n"), "demo"); err == nil {
		t.Error("want error when modules key missing")
	}
}
