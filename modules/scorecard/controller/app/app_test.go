package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClaims_MissingToken401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	ctrl := &AppController{}
	if _, ok := ctrl.claims(c); ok {
		t.Fatal("无 token 应拒绝")
	}
}
