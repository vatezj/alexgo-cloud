package starfuyu

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/starfuyu/controller/admin"
)

// RegisterDivinationDBRoutes 注册 Divination 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewDivinationController(...)
func RegisterDivinationDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminDivinationController) {
	g := r.Group("/admin/starfuyu")
	g.GET("/divinations", c.List)
	g.GET("/divinations/:id", c.Get)
	g.POST("/divinations", c.Create)
	g.PUT("/divinations/:id", c.Update)
	g.DELETE("/divinations/:id", c.Delete)
}
