package taokouling

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/taokouling/controller/admin"
)

// RegisterChannelDBRoutes 注册 Channel 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewChannelController(...)
func RegisterChannelDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminChannelController) {
	g := r.Group("/admin/taokouling")
	g.GET("/channels", c.List)
	g.GET("/channels/:id", c.Get)
	g.POST("/channels", c.Create)
	g.PUT("/channels/:id", c.Update)
	g.DELETE("/channels/:id", c.Delete)
}
