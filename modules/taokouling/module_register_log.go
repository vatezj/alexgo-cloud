package taokouling

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/taokouling/controller/admin"
)

// RegisterLogDBRoutes 注册 Log 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewLogController(...)
// T4b 机械适配：新仓储仅 Append/List，Get/Update/:id 路由随死代码删除（T13/T14 重写时整体替换）。
func RegisterLogDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminLogController) {
	g := r.Group("/admin/taokouling")
	g.GET("/logs", c.List)
	g.POST("/logs", c.Create)
}
