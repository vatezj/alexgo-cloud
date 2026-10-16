package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newDriver(t *testing.T) (*LocalDriver, string) {
	t.Helper()
	root := t.TempDir()
	d, err := NewLocalDriver(root)
	if err != nil {
		t.Fatal(err)
	}
	return d, root
}

// Save→Open 字节往返；Delete 后 Open 报 ErrNotFound。
func TestLocalDriver_RoundTripAndDelete(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	if err := d.Save(ctx, "2026/10/aaaa.bin", strings.NewReader("hello")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	rc, err := d.Open(ctx, "2026/10/aaaa.bin")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "hello" {
		t.Errorf("content = %q, want hello", b)
	}
	if err := d.Delete(ctx, "2026/10/aaaa.bin"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Open(ctx, "2026/10/aaaa.bin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open after delete err = %v, want ErrNotFound", err)
	}
	if err := d.Delete(ctx, "2026/10/aaaa.bin"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete twice err = %v, want ErrNotFound", err)
	}
}

// 路径穿越双保险：../ 逃逸与绝对路径注入都必须被拒，且不得在 root 外落文件。
func TestLocalDriver_TraversalRejected(t *testing.T) {
	d, root := newDriver(t)
	ctx := context.Background()
	for _, rel := range []string{
		"../../etc/passwd",
		"2026/10/../../../outside.txt",
		filepath.Join(root, "injected-abs.txt"),
	} {
		if err := d.Save(ctx, rel, strings.NewReader("x")); err == nil {
			t.Errorf("Save(%q) succeeded, want error", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside.txt")); !os.IsNotExist(err) {
		t.Errorf("file escaped temp root: err=%v", err)
	}
}

// O_EXCL 防覆盖：同 relPath 二次 Save 报错且首份内容不变。
func TestLocalDriver_NoOverwrite(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()
	if err := d.Save(ctx, "a/b.bin", strings.NewReader("first")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := d.Save(ctx, "a/b.bin", strings.NewReader("second")); err == nil {
		t.Fatal("second Save succeeded, want O_EXCL error")
	}
	rc, err := d.Open(ctx, "a/b.bin")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "first" {
		t.Errorf("content = %q, want first (must not overwrite)", b)
	}
}

// Save 写入中途失败不留半截文件（io.Reader 中途报错）。
func TestLocalDriver_SaveHalfWriteCleaned(t *testing.T) {
	d, root := newDriver(t)
	if err := d.Save(context.Background(), "x/half.bin", errReader{}); err == nil {
		t.Fatal("Save with failing reader succeeded, want error")
	}
	var leftovers []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) != 0 {
		t.Errorf("leftover files: %v", leftovers)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
