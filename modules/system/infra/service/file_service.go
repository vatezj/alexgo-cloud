package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/repository"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/storage"
	"alexGo-cloud/pkg/tenant"
)

// defaultAllowedExts 内置白名单（spec §3/§4）：刻意不含 svg/html（存储型 XSS 面）。
var defaultAllowedExts = []string{"jpg", "jpeg", "png", "gif", "webp", "pdf", "zip", "xlsx"}

// extMimeOK 扩展名 → DetectContentType 允许值集合（双 MIME 校验的第二半）。
// zip/xlsx 都是 PK 容器，探测值同为 application/zip。
var extMimeOK = map[string][]string{
	"jpg":  {"image/jpeg"},
	"jpeg": {"image/jpeg"},
	"png":  {"image/png"},
	"gif":  {"image/gif"},
	"webp": {"image/webp"},
	"pdf":  {"application/pdf"},
	"zip":  {"application/zip"},
	"xlsx": {"application/zip"},
}

// UploadInput controller 侧收集的上传输入。
// Size 只作预检快拒——最终以字节流限流为准（FileHeader.Size 客户端可伪造）。
type UploadInput struct {
	Name    string // 原始文件名（含扩展名）
	Size    int64  // 声明大小（预检）
	Reader  io.Reader
	Public  bool
	BizType string // 仅归类元数据，不校验枚举
}

// FileService 文件元数据业务：策略（随机名/白名单/大小/双 MIME）+ 落库 + 驱动编排。
type FileService interface {
	Upload(ctx context.Context, in UploadInput) (*model.SystemFile, error)
	GetByID(ctx context.Context, id uint64) (*model.SystemFile, error)
	// GetByKey public 读入口：private/不存在统一 CodeFileNotFound（不泄露存在性）。
	GetByKey(ctx context.Context, key string) (*model.SystemFile, error)
	Open(ctx context.Context, f *model.SystemFile) (io.ReadCloser, error)
	Delete(ctx context.Context, id uint64) error
}

type fileService struct {
	repo   repository.FileRepository
	driver storage.Driver
	max    int64 // 字节上限
	exts   map[string]struct{}
}

// NewFileService 构造文件服务。maxSizeMB<=0 落默认 10（config 缺省防御）。
func NewFileService(repo repository.FileRepository, driver storage.Driver, cfg *config.Config) FileService {
	mb := 10
	exts := defaultAllowedExts
	if cfg != nil {
		if cfg.Storage.MaxSizeMB > 0 {
			mb = cfg.Storage.MaxSizeMB
		}
		if len(cfg.Storage.AllowedExts) > 0 {
			exts = cfg.Storage.AllowedExts
		}
	}
	set := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		set[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	return &fileService{repo: repo, driver: driver, max: int64(mb) * 1024 * 1024, exts: set}
}

// Upload 全流程（spec §5）：白名单 → 大小双保险 → 头探测双 MIME → 生成路径 →
// Save（限流字节）→ 超限补偿删 → 落库（失败盘删补偿）。
func (s *fileService) Upload(ctx context.Context, in UploadInput) (*model.SystemFile, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(in.Name), "."))
	if ext == "" {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("no extension in %q", in.Name))
	}
	if _, ok := s.exts[ext]; !ok {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("ext %q not in allowlist", ext))
	}
	if in.Size > s.max {
		return nil, errs.New(errs.CodeFileTooLarge, fmt.Errorf("declared %d > max %d", in.Size, s.max))
	}

	// 头 512 字节探测：Peek 不消费，后续经同一 bufio 流入 Save。
	br := bufio.NewReaderSize(in.Reader, 512)
	head, _ := br.Peek(512)
	sniffed := http.DetectContentType(head)
	allowedMimes, known := extMimeOK[ext]
	if !known {
		return nil, errs.New(errs.CodeFileTypeNotAllowed, fmt.Errorf("no mime mapping for ext %q", ext))
	}
	matched := false
	for _, m := range allowedMimes {
		if sniffed == m {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errs.New(errs.CodeFileTypeNotAllowed,
			fmt.Errorf("sniffed %q mismatches ext %q", sniffed, ext))
	}

	relPath, publicKey, err := s.genPaths(ext)
	if err != nil {
		return nil, err
	}

	// 字节级限流：写 max+1 字节后仍可读 → 超限，删掉刚写的文件再拒。
	lr := &io.LimitedReader{R: br, N: s.max + 1}
	cr := &countingReader{r: lr}
	if err := s.driver.Save(ctx, relPath, cr); err != nil {
		return nil, err // O_EXCL 碰撞/IO 裸错 → resp.Fail 兜底 50000
	}
	if cr.n > s.max {
		if derr := s.driver.Delete(ctx, relPath); derr != nil && !errors.Is(derr, storage.ErrNotFound) {
			warnf("oversize cleanup failed", zap.String("rel_path", relPath), zap.Error(derr))
		}
		return nil, errs.New(errs.CodeFileTooLarge, fmt.Errorf("stream %d > max %d", cr.n, s.max))
	}

	f := &model.SystemFile{
		Name: in.Name, Ext: ext, Mime: sniffed, Size: cr.n,
		RelPath: relPath, PublicKey: publicKey,
		Public: in.Public, BizType: in.BizType,
		TenantID: tenant.TenantIDFromContext(ctx), CreatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, f); err != nil {
		// 补偿：盘删尽力；删不掉只留 warn（孤儿路径靠日志人工/后续 GC）。
		if derr := s.driver.Delete(ctx, relPath); derr != nil && !errors.Is(derr, storage.ErrNotFound) {
			warnf("orphan file after db failure", zap.String("rel_path", relPath), zap.Error(derr))
		}
		return nil, err
	}
	return f, nil
}

// genPaths 分桶相对路径 + 独立 public_key（均 crypto/rand 16 字节 hex）。
func (s *fileService) genPaths(ext string) (relPath, publicKey string, err error) {
	id, err := randHex()
	if err != nil {
		return "", "", err
	}
	key, err := randHex()
	if err != nil {
		return "", "", err
	}
	return time.Now().Format("2006/01") + "/" + id + "." + ext, key, nil
}

func randHex() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("storage: rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *fileService) GetByID(ctx context.Context, id uint64) (*model.SystemFile, error) {
	f, err := s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	return f, nil
}

func (s *fileService) GetByKey(ctx context.Context, key string) (*model.SystemFile, error) {
	f, err := s.repo.GetByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	if !f.Public {
		// private 行与不存在同码（30010），不泄露存在性（spec §7）。
		return nil, errs.New(errs.CodeFileNotFound, nil)
	}
	return f, nil
}

func (s *fileService) Open(ctx context.Context, f *model.SystemFile) (io.ReadCloser, error) {
	rc, err := s.driver.Open(ctx, f.RelPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, errs.New(errs.CodeFileNotFound, nil)
		}
		return nil, err
	}
	return rc, nil
}

func (s *fileService) Delete(ctx context.Context, id uint64) error {
	f, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenant.TenantIDFromContext(ctx), id); err != nil {
		return err
	}
	// 行删成、盘删败 → 不回滚不报错，warn 留 rel_path（spec §7，GC 后续票）。
	if err := s.driver.Delete(ctx, f.RelPath); err != nil && !errors.Is(err, storage.ErrNotFound) {
		warnf("disk delete failed", zap.String("rel_path", f.RelPath), zap.Error(err))
	}
	return nil
}

// countingReader 统计实际写入字节数（超限判定依据）。
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// warnf 日志缺省不 panic——logger 未初始化时静默跳过。
func warnf(msg string, fields ...zap.Field) {
	if logger.Log != nil {
		logger.Log.Warn(msg, fields...)
	}
}
