package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterRoomDBRoutes 注册 Room 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewRoomController(...)
func RegisterRoomDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminRoomController) {
	g := r.Group("/admin/scorecard")
	g.GET("/rooms", c.List)
	g.GET("/rooms/:id", c.Get)
	g.POST("/rooms", c.Create)
	g.PUT("/rooms/:id", c.Update)
	g.DELETE("/rooms/:id", c.Delete)
}
