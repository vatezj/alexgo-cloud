package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
	"alexGo-cloud/pkg/storage"
)

func newPublicCtrl(t *testing.T) (*PublicController, *service.ShareService) {
	t.Helper()
	db := newAppTestDB(t)
	driver, err := storage.NewLocalDriver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewShareService(repository.NewShareRepository(db), driver, "")
	return NewPublicController(svc), svc
}

// TestPublicShare_404 公开读 miss → 裸 404；命中 → 200 inline（spec §4.5）。
func TestPublicShare_404AndHit(t *testing.T) {
	pub, svc := newPublicCtrl(t)
	body := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3}
	url, err := svc.Upload(t.Context(), 1, "card.png", "https://h.example", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	key := url[strings.LastIndex(url, "/")+1:]

	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	c.Params = gin.Params{{Key: "key", Value: "missing"}}
	pub.Share(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("miss 应 404, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	c2.Params = gin.Params{{Key: "key", Value: key}}
	pub.Share(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("命中应 200, got %d", w2.Code)
	}
	if ct := w2.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("应 image/png, got %s", ct)
	}
	if !strings.Contains(w2.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("应 inline: %s", w2.Header().Get("Content-Disposition"))
	}
	if !bytes.Equal(w2.Body.Bytes(), body) {
		t.Fatalf("图字节应一致: len=%d", w2.Body.Len())
	}
}
