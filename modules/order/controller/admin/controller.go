package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/excel"
	"alexGo-cloud/pkg/resp"
)

type AdminOrderController struct {
	orderSvc service.OrderService
}

func NewAdminOrderController(orderSvc service.OrderService) *AdminOrderController {
	return &AdminOrderController{orderSvc: orderSvc}
}

type createOrderRequest struct {
	UserID    uint64 `json:"user_id"`
	ProductID uint64 `json:"product_id"`
	Amount    int64  `json:"amount"`
	Status    int    `json:"status"`
	OrderNo   string `json:"order_no"`
}

func (c *AdminOrderController) Create(ctx *gin.Context) {
	var req createOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	entity := &model.Order{
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Amount:    req.Amount,
		Status:    req.Status,
		OrderNo:   req.OrderNo,
	}
	if err := c.orderSvc.Create(ctx.Request.Context(), entity); err != nil {
		resp.Fail(ctx, err) // 写库类 → 500 裸透传
		return
	}
	resp.OK(ctx, nil)
}

// List 全量列表（无 page/size 入参）→ resp.OK 裸数组，不顺手加分页。
func (c *AdminOrderController) List(ctx *gin.Context) {
	data, err := c.orderSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, data)
}

func (c *AdminOrderController) CreateOrder(ctx *gin.Context) {
	c.Create(ctx)
}

func (c *AdminOrderController) ListOrders(ctx *gin.Context) {
	c.List(ctx)
}

// ExportOrders 导出当前租户全量订单 xlsx（GET /orders/export）。
// 复用既有全量 List（租户插件经 ctx 生效）；Build 失败字节未写，信封安全（RF②）。
func (c *AdminOrderController) ExportOrders(ctx *gin.Context) {
	orders, err := c.orderSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	spec := excel.Spec[*model.Order]{
		Sheet:    "订单",
		FileName: excel.FileName("orders"),
		Columns: []excel.Column[*model.Order]{
			{Title: "ID", Cell: func(o *model.Order) any { return o.ID }},
			{Title: "订单号", Cell: func(o *model.Order) any { return o.OrderNo }},
			{Title: "用户ID", Cell: func(o *model.Order) any { return o.UserID }},
			{Title: "产品ID", Cell: func(o *model.Order) any { return o.ProductID }},
			{Title: "金额(分)", Cell: func(o *model.Order) any { return o.Amount }},
			{Title: "状态", Cell: func(o *model.Order) any { return o.Status }},
			{Title: "创建时间", Cell: func(o *model.Order) any { return excel.TimeRFC3339(&o.CreatedAt) }},
			{Title: "更新时间", Cell: func(o *model.Order) any { return excel.TimeRFC3339(&o.UpdatedAt) }},
		},
		Rows: orders,
	}
	data, err := excel.Build(spec)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", excel.Disposition(spec.FileName))
	ctx.Data(http.StatusOK, excel.MIME, data)
}
