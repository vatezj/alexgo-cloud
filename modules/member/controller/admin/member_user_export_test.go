package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/service"
	"alexGo-cloud/pkg/excel"
)

// stubExportSvc：只关心 Export 的桩（嵌入接口，照 stubListSvc 模式）。
type stubExportSvc struct {
	service.MemberService
	members []*model.MemberUser
	err     error
}

func (s *stubExportSvc) Export(context.Context) ([]*model.MemberUser, error) {
	return s.members, s.err
}

func TestExportMemberUsers_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	svc := &stubExportSvc{members: []*model.MemberUser{{
		ID: 1, Nickname: "会员甲", Mobile: "13811112222", Status: 1,
		RegisterIP: "5.6.7.8", LoginIP: "1.2.3.4", LoginDate: &when, CreatedAt: when,
	}}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewUserController(svc)
	r.GET("/api/admin/member/users/export", c.ExportMemberUsers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/member/users/export", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "member_users_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("会员")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][7] != "创建时间" || len(rows[0]) != 8 {
		t.Errorf("header = %v, want 8 cols ID..创建时间", rows[0])
	}
	if rows[1][1] != "会员甲" || rows[1][2] != "13811112222" {
		t.Errorf("row1 = %v", rows[1])
	}
}
