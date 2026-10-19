package newmodule

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "更新 golden")

// TestGoldenPaths 钉住完整 ChangeSet 的文件清单（防触点增漏）。
// 清单变化时：先确认是预期变更，再更新 testdata/golden_paths.txt。
func TestGoldenPaths(t *testing.T) {
	o := tempOpts(t) // demo / product+category / 2026-10-19
	cs, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cs.Creates {
		got = append(got, "C "+c.Path)
	}
	for _, p := range cs.Patches {
		got = append(got, "P "+p.Path)
	}
	sort.Strings(got)

	goldenPath := filepath.Join("testdata", "golden_paths.txt")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("写 golden: %v", err)
		}
		return
	}
	wantRaw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("读 golden: %v（首次运行先生成：go test -run TestGoldenPaths -update）", err)
	}
	var want []string
	for _, line := range strings.Split(string(wantRaw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			want = append(want, line)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("路径数 %d != golden %d\ngot:\n%s\nwant:\n%s", len(got), len(want),
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("路径[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
