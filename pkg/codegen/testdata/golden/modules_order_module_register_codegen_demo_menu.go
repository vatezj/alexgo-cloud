package order

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/order/controller/admin"
)

// RegisterCodegenDemoMenuDBRoutes 注册 CodegenDemoMenu 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewCodegenDemoMenuController(...)
func RegisterCodegenDemoMenuDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminCodegenDemoMenuController) {
	g := r.Group("/admin/order")
	g.GET("/codegen_demo_menus", c.List)
	g.GET("/codegen_demo_menus/:id", c.Get)
	g.POST("/codegen_demo_menus", c.Create)
	g.PUT("/codegen_demo_menus/:id", c.Update)
	g.DELETE("/codegen_demo_menus/:id", c.Delete)
}
