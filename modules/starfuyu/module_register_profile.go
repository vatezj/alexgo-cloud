package starfuyu

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/starfuyu/controller/admin"
)

// RegisterProfileDBRoutes 注册 Profile 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewProfileController(...)
func RegisterProfileDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminProfileController) {
	g := r.Group("/admin/starfuyu")
	g.GET("/profiles", c.List)
	g.GET("/profiles/:id", c.Get)
	g.POST("/profiles", c.Create)
	g.PUT("/profiles/:id", c.Update)
	g.DELETE("/profiles/:id", c.Delete)
}
