package taokouling

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/taokouling/controller/admin"
)

// RegisterLogDBRoutes 注册 Log 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewLogController(...)
func RegisterLogDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminLogController) {
	g := r.Group("/admin/taokouling")
	g.GET("/logs", c.List)
	g.GET("/logs/:id", c.Get)
	g.POST("/logs", c.Create)
	g.PUT("/logs/:id", c.Update)
	g.DELETE("/logs/:id", c.Delete)
}
