package order

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"alexGo-cloud/alexgo-server/server"
	"alexGo-cloud/modules/order/controller/admin"
	"alexGo-cloud/modules/order/repository"
	"alexGo-cloud/modules/order/service"
	"alexGo-cloud/pkg/config"
)

var FxModule = fx.Module("order",
	fx.Provide(
		repository.NewOrderRepository,
		service.NewOrderService,
		admin.NewAdminOrderController,
	),
	fx.Provide(
		fx.Annotate(
			NewOrderModule,
			fx.As(new(server.Module)),
			fx.ResultTags(`group:"modules"`),
		),
	),
)

type orderModule struct {
	cfg  *config.Config
	ctrl *admin.AdminOrderController
}

type OrderModuleParams struct {
	fx.In

	Cfg  *config.Config
	Ctrl *admin.AdminOrderController
}

func NewOrderModule(p OrderModuleParams) server.Module {
	return &orderModule{cfg: p.Cfg, ctrl: p.Ctrl}
}

func (m *orderModule) RegisterRoutes(r *gin.RouterGroup) {
	if m.cfg != nil {
		if on, ok := m.cfg.Modules["order"]; ok && !on {
			return
		}
	}
	adminGroup := r.Group("/admin/order")
	adminGroup.GET("/orders", m.ctrl.ListOrders)
	adminGroup.POST("/orders", m.ctrl.CreateOrder)
}
