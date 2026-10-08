package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/modules/order/service"
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *AdminOrderController) List(ctx *gin.Context) {
	data, err := c.orderSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

func (c *AdminOrderController) CreateOrder(ctx *gin.Context) {
	c.Create(ctx)
}

func (c *AdminOrderController) ListOrders(ctx *gin.Context) {
	c.List(ctx)
}
