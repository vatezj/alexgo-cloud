package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
)

// newFileSvc 真 sqlite repo + 真 LocalDriver(t.TempDir())，cfg 可调大小上限。
func newFileSvc(t *testing.T, maxSizeMB int) (FileService, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemFile{}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	drv, err := storage.NewLocalDriver(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Storage.MaxSizeMB = maxSizeMB
	return NewFileService(repository.NewFileRepository(db), drv, cfg), db, root
}

// jpegBytes 头 3 字节 FF D8 FF——DetectContentType 判 image/jpeg。
func jpegBytes(n int) []byte {
	b := bytes.Repeat([]byte{0x00}, n)
	copy(b, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return b
}

var relPathRe = regexp.MustCompile(`^\d{4}/\d{2}/[0-9a-f]{32}\.[a-z]+$`)

// 成功路径：分桶 rel_path、32hex public_key、落库字段齐全、盘上有真实字节。
func TestUpload_Success(t *testing.T) {
	svc, db, root := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)

	meta, err := svc.Upload(ctx, UploadInput{
		Name: "照片.JPG", Size: 4, Reader: bytes.NewReader(jpegBytes(4)),
		Public: true, BizType: "notice",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !relPathRe.MatchString(meta.RelPath) {
		t.Errorf("RelPath = %q, want yyyy/mm/<32hex>.<ext>", meta.RelPath)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(meta.PublicKey) {
		t.Errorf("PublicKey = %q, want 32 hex", meta.PublicKey)
	}
	if meta.Ext != "jpg" || meta.Mime != "image/jpeg" || meta.Size != 4 {
		t.Errorf("ext/mime/size = %q/%q/%d, want jpg/image/jpeg/4", meta.Ext, meta.Mime, meta.Size)
	}
	if meta.TenantID != 1 || !meta.Public || meta.BizType != "notice" {
		t.Errorf("tenant/public/biz = %d/%v/%q, want 1/true/notice", meta.TenantID, meta.Public, meta.BizType)
	}
	// 盘上有字节
	b, err := os.ReadFile(filepath.Join(root, meta.RelPath))
	if err != nil || len(b) != 4 {
		t.Errorf("disk read: n=%d err=%v", len(b), err)
	}
	// 落库
	var row model.SystemFile
	if err := db.First(&row, meta.ID).Error; err != nil {
		t.Fatalf("db row: %v", err)
	}
	if row.PublicKey != meta.PublicKey {
		t.Errorf("db PublicKey = %q, want %q", row.PublicKey, meta.PublicKey)
	}
}

// 双 MIME 拒：扩展名不在白名单（svg）与头探测不符（jpg 装 gif 头）都是 30011。
func TestUpload_MimeRejections_30011(t *testing.T) {
	svc, _, _ := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)
	for _, tc := range []struct {
		name string
		in   UploadInput
	}{
		{"svg 白名单拒绝", UploadInput{Name: "a.svg", Reader: bytes.NewReader([]byte("<svg></svg>"))}},
		{"扩展名与探测头不符", UploadInput{Name: "a.jpg", Reader: bytes.NewReader([]byte("GIF89a...."))}},
		{"无扩展名", UploadInput{Name: "noext", Reader: bytes.NewReader(jpegBytes(4))}},
	} {
		if _, err := svc.Upload(ctx, tc.in); !errs.Is(err, errs.CodeFileTypeNotAllowed) {
			t.Errorf("%s: err = %v, want CodeFileTypeNotAllowed", tc.name, err)
		}
	}
}

// 超限 30012——Review Focus ②：Size 声明撒谎（传 0），实际字节流超 max 必须被拒，
// 且盘上不得残留半截文件（字节级限流，而非只信 FileHeader.Size）。
func TestUpload_ByteStreamLimited_DeclaredSizeLies_30012(t *testing.T) {
	svc, _, root := newFileSvc(t, 1) // max = 1MB
	ctx := tenant.WithTenantID(context.Background(), 1)
	over := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0}, (1<<20)/4+1) // > 1MB
	if _, err := svc.Upload(ctx, UploadInput{Name: "big.jpg", Size: 0, Reader: bytes.NewReader(over)}); !errs.Is(err, errs.CodeFileTooLarge) {
		t.Fatalf("oversize stream err = %v, want CodeFileTooLarge", err)
	}
	// 盘上不得残留
	n := countFiles(t, root)
	if n != 0 {
		t.Errorf("leftover files = %d, want 0 (over-limit write must be removed)", n)
	}
	// 声明值撒谎的另一半：Size 声明超限也直接拒
	if _, err := svc.Upload(ctx, UploadInput{Name: "big.jpg", Size: 2 << 20, Reader: bytes.NewReader(jpegBytes(4))}); !errs.Is(err, errs.CodeFileTooLarge) {
		t.Errorf("declared oversize err = %v, want CodeFileTooLarge", err)
	}
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// 租户边界（Review Focus ①）：租户 1 上传后，租户 99 的 ctx 按 key 仍能 GetByKey 命中；
// private 行 GetByKey 统一 30010；GetByID 跨租户 30010。
func TestUpload_GetByKey_TenantBoundary(t *testing.T) {
	svc, _, _ := newFileSvc(t, 1)
	ctx1 := tenant.WithTenantID(context.Background(), 1)

	pub, err := svc.Upload(ctx1, UploadInput{Name: "p.jpg", Reader: bytes.NewReader(jpegBytes(4)), Public: true})
	if err != nil {
		t.Fatal(err)
	}
	priv, err := svc.Upload(ctx1, UploadInput{Name: "q.jpg", Reader: bytes.NewReader(jpegBytes(4)), Public: false})
	if err != nil {
		t.Fatal(err)
	}

	ctx99 := tenant.WithTenantID(context.Background(), 99)
	got, err := svc.GetByKey(ctx99, pub.PublicKey) // 跨租户 ctx 必须命中
	if err != nil || got.ID != pub.ID {
		t.Errorf("GetByKey cross tenant ctx: got=%+v err=%v, want hit", got, err)
	}
	// private 行：统一 30010（不泄露存在性）
	if _, err := svc.GetByKey(ctx99, priv.PublicKey); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByKey private err = %v, want CodeFileNotFound(30010)", err)
	}
	// 跨租户受控读 30010
	if _, err := svc.GetByID(ctx99, pub.ID); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByID cross tenant err = %v, want CodeFileNotFound", err)
	}
	// key 查无 → 30010
	if _, err := svc.GetByKey(ctx99, strings.Repeat("f", 32)); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("GetByKey missing err = %v, want CodeFileNotFound", err)
	}
}

// Delete：行删 + 盘删都生效；盘文件被预先移走（模拟盘删未命中）→ 仍返回 nil
// （spec §7：行删成盘删败不报错，warn 留 rel_path 给后续 GC）。
func TestUpload_Delete_RowAndDisk(t *testing.T) {
	svc, db, root := newFileSvc(t, 1)
	ctx := tenant.WithTenantID(context.Background(), 1)
	meta, err := svc.Upload(ctx, UploadInput{Name: "d.jpg", Reader: bytes.NewReader(jpegBytes(4))})
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, meta.RelPath)
	if err := os.Remove(disk); err != nil { // 先抽掉盘文件
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, meta.ID); err != nil {
		t.Errorf("Delete with missing disk file err = %v, want nil (warn only)", err)
	}
	var n int64
	db.Model(&model.SystemFile{}).Count(&n)
	if n != 0 {
		t.Errorf("row count = %d, want 0", n)
	}
	// 再删一次：行已无 → 30010
	if err := svc.Delete(ctx, meta.ID); !errs.Is(err, errs.CodeFileNotFound) {
		t.Errorf("Delete twice err = %v, want CodeFileNotFound", err)
	}
}

// 落库失败补偿：repo.Create 报错 → 盘上不得留孤儿文件（spec §7）。
func TestUpload_CreateFails_DiskCompensated(t *testing.T) {
	drvRoot := t.TempDir()
	drv, err := storage.NewLocalDriver(drvRoot)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Storage.MaxSizeMB = 1
	svc := NewFileService(failingCreateRepo{}, drv, cfg)
	ctx := tenant.WithTenantID(context.Background(), 1)
	if _, err := svc.Upload(ctx, UploadInput{Name: "o.jpg", Reader: bytes.NewReader(jpegBytes(4))}); err == nil {
		t.Fatal("Upload with failing repo succeeded, want error")
	}
	if n := countFiles(t, drvRoot); n != 0 {
		t.Errorf("orphan files = %d, want 0 (compensated)", n)
	}
}

// failingCreateRepo Create 恒失败，其余方法不被调用。
type failingCreateRepo struct{}

func (failingCreateRepo) Create(context.Context, *model.SystemFile) error {
	return errors.New("db down")
}
func (failingCreateRepo) GetByID(context.Context, uint64, uint64) (*model.SystemFile, error) {
	return nil, errors.New("unused")
}
func (failingCreateRepo) GetByKey(context.Context, string) (*model.SystemFile, error) {
	return nil, errors.New("unused")
}
func (failingCreateRepo) Delete(context.Context, uint64, uint64) error { return errors.New("unused") }

var _ io.Reader = bytes.NewReader(nil) // 保持 io 导入被使用（Reader 合同）
