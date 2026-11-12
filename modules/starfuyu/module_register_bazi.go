package starfuyu

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/starfuyu/controller/admin"
)

// RegisterBaziDBRoutes 注册 Bazi 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewBaziController(...)
func RegisterBaziDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminBaziController) {
	g := r.Group("/admin/starfuyu")
	g.GET("/bazis", c.List)
	g.GET("/bazis/:id", c.Get)
	g.POST("/bazis", c.Create)
	g.PUT("/bazis/:id", c.Update)
	g.DELETE("/bazis/:id", c.Delete)
}
