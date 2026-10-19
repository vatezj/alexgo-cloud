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

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/service"
	"alexGo-cloud/pkg/excel"
)

// stubOrderSvc：只关心 List 的桩（嵌入接口，Create 不触达）。
type stubOrderSvc struct {
	service.OrderService
	orders []*model.Order
	err    error
}

func (s *stubOrderSvc) List(context.Context) ([]*model.Order, error) {
	return s.orders, s.err
}

func TestExportOrders_XlsxRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	svc := &stubOrderSvc{orders: []*model.Order{{
		ID: 9, OrderNo: "ORD-001", UserID: 1, ProductID: 2,
		Amount: 19900, Status: 1, CreatedAt: when, UpdatedAt: when,
	}}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAdminOrderController(svc)
	r.GET("/api/admin/order/orders/export", c.ExportOrders)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/order/orders/export", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != excel.MIME {
		t.Errorf("Content-Type = %q, want %q", ct, excel.MIME)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "orders_") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("订单")
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0] != "ID" || rows[0][1] != "订单号" || rows[0][4] != "金额(分)" || len(rows[0]) != 8 {
		t.Errorf("header = %v, want 8 cols ID..更新时间", rows[0])
	}
	// 金额导原始 int64 分：19900 → "19900"。
	if rows[1][1] != "ORD-001" || rows[1][4] != "19900" {
		t.Errorf("row1 = %v", rows[1])
	}
}

// 空表：仅表头 200 合法（spec §3）。
func TestExportOrders_Empty_HeaderOnly200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAdminOrderController(&stubOrderSvc{})
	r.GET("/api/admin/order/orders/export", c.ExportOrders)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/order/orders/export", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	rows, _ := f.GetRows("订单")
	if len(rows) != 1 {
		t.Errorf("rows = %d, want header only", len(rows))
	}
}
