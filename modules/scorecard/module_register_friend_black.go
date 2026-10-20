package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterFriendBlackDBRoutes 注册 FriendBlack 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewFriendBlackController(...)
func RegisterFriendBlackDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminFriendBlackController) {
	g := r.Group("/admin/scorecard")
	g.GET("/friend_blacks", c.List)
	g.GET("/friend_blacks/:id", c.Get)
	g.POST("/friend_blacks", c.Create)
	g.PUT("/friend_blacks/:id", c.Update)
	g.DELETE("/friend_blacks/:id", c.Delete)
}
