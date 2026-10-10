package order

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/order/controller/admin"
)

// RegisterCodegenDemoItemDBRoutes 注册 CodegenDemoItem 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewCodegenDemoItemController(...)
func RegisterCodegenDemoItemDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminCodegenDemoItemController) {
	g := r.Group("/admin/order")
	g.GET("/codegen_demo_items", c.List)
	g.GET("/codegen_demo_items/:id", c.Get)
	g.POST("/codegen_demo_items", c.Create)
	g.PUT("/codegen_demo_items/:id", c.Update)
	g.DELETE("/codegen_demo_items/:id", c.Delete)
}
