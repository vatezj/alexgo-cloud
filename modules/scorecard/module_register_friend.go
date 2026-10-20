package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterFriendDBRoutes 注册 Friend 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewFriendController(...)
func RegisterFriendDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminFriendController) {
	g := r.Group("/admin/scorecard")
	g.GET("/friends", c.List)
	g.GET("/friends/:id", c.Get)
	g.POST("/friends", c.Create)
	g.PUT("/friends/:id", c.Update)
	g.DELETE("/friends/:id", c.Delete)
}
