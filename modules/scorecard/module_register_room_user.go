package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterRoomUserDBRoutes 注册 RoomUser 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewRoomUserController(...)
func RegisterRoomUserDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminRoomUserController) {
	g := r.Group("/admin/scorecard")
	g.GET("/room_users", c.List)
	g.GET("/room_users/:id", c.Get)
	g.POST("/room_users", c.Create)
	g.PUT("/room_users/:id", c.Update)
	g.DELETE("/room_users/:id", c.Delete)
}
