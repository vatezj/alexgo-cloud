package scorecard

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/scorecard/controller/admin"
)

// RegisterRecordDBRoutes 注册 Record 的 admin 路由（生成文件，勿手改）。
// 由 module.go 手动调用接线：adminctrl := admin.NewRecordController(...)
func RegisterRecordDBRoutes(r *gin.RouterGroup, c *adminctrl.AdminRecordController) {
	g := r.Group("/admin/scorecard")
	g.GET("/records", c.List)
	g.GET("/records/:id", c.Get)
	g.POST("/records", c.Create)
	g.PUT("/records/:id", c.Update)
	g.DELETE("/records/:id", c.Delete)
}
