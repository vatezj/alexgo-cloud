package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterRoomLogDBRoutes 注册 RoomLog 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewRoomLogController(...)
func RegisterRoomLogDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminRoomLogController) {
	g := r.Group("/admin/scorecard")
	g.GET("/room_logs", c.List)
	g.GET("/room_logs/:id", c.Get)
	g.POST("/room_logs", c.Create)
	g.PUT("/room_logs/:id", c.Update)
	g.DELETE("/room_logs/:id", c.Delete)
}
