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
		t.Error("缺少 demo: true")
	}
	// 既有行与注释保留（抽查）
	for _, keep := range []string{"system: true", "order: true", "# 盘根"} {
		if !strings.Contains(got, keep) {
			t.Errorf("丢失内容 %q", keep)
		}
	}
	// 字节级保真：输出必须 = 原文件恰好插入一行；删掉插入行后与原字节全等（重排必须红）
	outLines := strings.Split(got, "\n")
	srcLines := strings.Split(string(src), "\n")
	if len(outLines) != len(srcLines)+1 {
		t.Errorf("行数应为原行数+1: got %d want %d", len(outLines), len(srcLines)+1)
	}
	kept := make([]string, 0, len(outLines))
	found := 0
	for _, l := range outLines {
		if strings.TrimSpace(l) == "demo: true" {
			found++
			continue
		}
		kept = append(kept, l)
	}
	if found != 1 {
		t.Fatalf("插入行出现 %d 次，应为 1 次", found)
	}
	if strings.Join(kept, "\n") != string(src) {
		t.Error("输出不是原文件恰好插入一行（发生了重排）")
	}
	// 重名拒绝
	if _, err := editConfigYAML(out, "demo"); err == nil {
		t.Error("重名插入应报错")
	}
}

func TestEditConfigYAML_MissingModules(t *testing.T) {
	if _, err := editConfigYAML([]byte("system:\n  dsn: x\n"), "demo"); err == nil {
		t.Error("缺少 modules 键时应报错")
	}
}
