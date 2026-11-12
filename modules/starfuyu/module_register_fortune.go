package starfuyu

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/starfuyu/controller/admin"
)

// RegisterFortuneDBRoutes 注册 Fortune 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewFortuneController(...)
func RegisterFortuneDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminFortuneController) {
	g := r.Group("/admin/starfuyu")
	g.GET("/fortunes", c.List)
	g.GET("/fortunes/:id", c.Get)
	g.POST("/fortunes", c.Create)
	g.PUT("/fortunes/:id", c.Update)
	g.DELETE("/fortunes/:id", c.Delete)
}
