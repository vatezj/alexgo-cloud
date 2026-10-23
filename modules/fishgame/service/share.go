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
	"strings"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/pkg/storage"
)

// 分享图上限（spec §4.5：≤5MB）与扩展名白名单。
const shareMaxBytes = 5 << 20

// Upload 失败哨兵（controller 映射 400 裸 error）。
var (
	ErrShareExt      = errors.New("file_type_not_allowed")
	ErrShareTooLarge = errors.New("file_too_large")
	ErrShareNotFound = errors.New("not_found")
)

// shareExtMime 扩展名 → DetectContentType 允许值（双 MIME 校验，照 file_service.go 策略）。
var shareExtMime = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"webp": "image/webp",
}

// ShareService 分享图托管：storage 驱动写盘 + fishgame_share 元数据（spec §4.5）。
type ShareService struct {
	shares     repository.ShareRepository
	driver     storage.Driver
	publicBase string // cfg.Fishgame.PublicBaseURL；空 = 调用方传 base
}

func NewShareService(shares repository.ShareRepository, driver storage.Driver, publicBase string) *ShareService {
	return &ShareService{shares: shares, driver: driver, publicBase: publicBase}
}

// Upload multipart 图落盘 + 元数据行，返回绝对 URL（base 由调用方给：配置优先，
// 空则 controller 用 scheme+Host 拼）。
func (s *ShareService) Upload(ctx context.Context, playerID uint64, filename, base string, r io.Reader) (string, error) {
	// 扩展名取最后一个点之后（服务端白名单先于任何落盘）
	i := strings.LastIndex(filename, ".")
	if i < 0 || i == len(filename)-1 {
		return "", ErrShareExt
	}
	ext := strings.ToLower(filename[i+1:])
	wantMime, ok := shareExtMime[ext]
	if !ok {
		return "", ErrShareExt
	}
	// 存盘扩展名归一（jpeg→jpg），与 Get 侧 shareExtOfMime 一致——否则上传返回的 URL 读 404。
	ext = shareExtOfMime(wantMime)

	// 头 512 字节双 MIME 探测（Peek 不消费，后续同一流传盘）。
	br := bufio.NewReaderSize(r, 512)
	head, _ := br.Peek(512)
	if sniffed := http.DetectContentType(head); sniffed != wantMime {
		return "", ErrShareExt
	}

	key, err := randomShareKey()
	if err != nil {
		return "", err
	}
	relPath := shareRelPath(key, ext)

	// 字节级限流：max+1 仍可读 → 超限，补偿删半截文件。
	lr := &io.LimitedReader{R: br, N: shareMaxBytes + 1}
	cr := &countingReader{r: lr}
	if err := s.driver.Save(ctx, relPath, cr); err != nil {
		return "", err
	}
	if cr.n > shareMaxBytes {
		if derr := s.driver.Delete(ctx, relPath); derr != nil && !errors.Is(derr, storage.ErrNotFound) {
			// 清理失败仅记不去重影响主流程（一期无 logger 注入，O_EXCL 是最终防线）
			_ = derr
		}
		return "", ErrShareTooLarge
	}

	if err := s.shares.Create(ctx, &model.Share{
		TenantID: 0,
		Key:      key,
		PlayerID: playerID,
		Size:     cr.n,
		Mime:     wantMime,
	}); err != nil {
		// 落库失败补偿删盘（对齐 file_service 策略）
		_ = s.driver.Delete(ctx, relPath)
		return "", err
	}

	if base == "" {
		base = s.publicBase
	}
	return strings.TrimRight(base, "/") + "/api/public/fishgame/share/" + key, nil
}

// Get 公开读：元数据 + 图字节；miss → ErrShareNotFound。
func (s *ShareService) Get(ctx context.Context, key string) (string, []byte, error) {
	row, err := s.shares.GetByKey(ctx, key)
	if err != nil {
		return "", nil, err
	}
	if row == nil {
		return "", nil, ErrShareNotFound
	}
	rc, err := s.driver.Open(ctx, shareRelPath(key, shareExtOfMime(row.Mime)))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", nil, ErrShareNotFound
		}
		return "", nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, shareMaxBytes+1))
	if err != nil {
		return "", nil, err
	}
	return row.Mime, data, nil
}

// shareRelPath 固定前缀 + 随机 key + 扩展名（驱动不得吃用户输入，key/映射都是服务端生成）。
func shareRelPath(key, ext string) string {
	return fmt.Sprintf("fishgame/share/%s.%s", key, ext)
}

// shareExtOfMime mime → 存盘扩展名（与 shareExtMime 反向对应，jpeg 统一存 jpg）。
func shareExtOfMime(mime string) string {
	switch mime {
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return "jpg"
	}
}

func randomShareKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// countingReader 统计写入字节（限流用，照 file_service 同名实现）。
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
