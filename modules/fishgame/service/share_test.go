package service

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/pkg/storage"
)

func newShareService(t *testing.T) (*ShareService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	driver, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewShareService(repository.NewShareRepository(db), driver, "")
	return svc, db
}

// pngBytes 最小 png 头 + 填充（DetectContentType 认 8 字节签名）。
func pngBytes(size int) []byte {
	sig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if size <= len(sig) {
		return sig
	}
	out := make([]byte, size)
	copy(out, sig)
	return out
}

func TestShareService_Upload_ExtNotAllowed(t *testing.T) {
	svc, _ := newShareService(t)
	_, err := svc.Upload(t.Context(), 1, "card.gif", "https://h.example", bytes.NewReader(pngBytes(64)))
	if !errors.Is(err, ErrShareExt) {
		t.Fatalf("应 ErrShareExt, got %v", err)
	}
}

func TestShareService_Upload_TooLarge(t *testing.T) {
	svc, _ := newShareService(t)
	_, err := svc.Upload(t.Context(), 1, "big.png", "https://h.example", bytes.NewReader(pngBytes(5*1024*1024+1)))
	if !errors.Is(err, ErrShareTooLarge) {
		t.Fatalf("应 ErrShareTooLarge, got %v", err)
	}
}

func TestShareService_Upload_ValidPng(t *testing.T) {
	svc, db := newShareService(t)
	body := pngBytes(128)
	url, err := svc.Upload(t.Context(), 1, "card.png", "https://h.example", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "/api/public/fishgame/share/") {
		t.Fatalf("url 应指向公开读端点: %s", url)
	}
	if !strings.HasPrefix(url, "https://h.example/") {
		t.Fatalf("绝对 URL 应带 base: %s", url)
	}
	key := url[strings.LastIndex(url, "/")+1:]

	var row model.Share
	if err := db.Where("`key` = ?", key).First(&row).Error; err != nil {
		t.Fatalf("share 行应落库: %v", err)
	}
	if row.Size != int64(len(body)) || row.Mime != "image/png" {
		t.Fatalf("元数据应齐: %+v", row)
	}

	gotMime, data, err := svc.Get(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	if gotMime != "image/png" || !bytes.Equal(data, body) {
		t.Fatalf("读回应一致: mime=%s len=%d", gotMime, len(data))
	}
}

func TestShareService_Get_Miss(t *testing.T) {
	svc, _ := newShareService(t)
	_, _, err := svc.Get(t.Context(), "nope")
	if !errors.Is(err, ErrShareNotFound) {
		t.Fatalf("应 ErrShareNotFound, got %v", err)
	}
}
