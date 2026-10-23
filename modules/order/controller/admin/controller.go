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

// CreateOrder godoc
//
//	@Summary		创建订单
//	@Description	权限 order:order；body 全字段平铺（user_id/product_id/amount 分/状态/订单号）。成功 data 为 null。
//	@Tags			order-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"订单信息"	Example({"user_id": 1, "product_id": 100, "amount": 9900, "status": 1, "order_no": "NO20261023001"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/order/orders [post]
func (c *AdminOrderController) CreateOrder(ctx *gin.Context) {
	c.Create(ctx)
}

// ListOrders godoc
//
//	@Summary		订单全量列表
//	@Description	权限 order:order；无 page/size 入参，data 为订单数组（不顺手加分页）。租户隔离在 repo 层（user_id → system_users.tenant_id）。
//	@Tags			order-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/order/orders [get]
func (c *AdminOrderController) ListOrders(ctx *gin.Context) {
	c.List(ctx)
}

// ExportOrders 导出当前租户全量订单 xlsx（GET /orders/export）。
// 复用既有 List：租户隔离在 repo 层显式做（user_id → system_users.tenant_id；
// Order 模型无 TenantID 字段，gormplugin 不接管）；Build 失败字节未写，信封安全（RF②）。

// ExportOrders godoc
//
//	@Summary		导出全量订单 xlsx
//	@Description	权限 order:order；当前租户全量订单 Excel（ID/订单号/用户/产品/金额分/状态/时间）。Content-Disposition 附件下载。
//	@Tags			order-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{file}	string
//	@Router			/admin/order/orders/export [get]
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
