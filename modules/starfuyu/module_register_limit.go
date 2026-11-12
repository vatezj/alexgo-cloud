package starfuyu

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/starfuyu/controller/admin"
)

// RegisterLimitDBRoutes 注册 Limit 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewLimitController(...)
func RegisterLimitDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminLimitController) {
	g := r.Group("/admin/starfuyu")
	g.GET("/limits", c.List)
	g.GET("/limits/:id", c.Get)
	g.POST("/limits", c.Create)
	g.PUT("/limits/:id", c.Update)
	g.DELETE("/limits/:id", c.Delete)
}
