package {{.Module}}

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/{{.Module}}/controller/admin"
	appctrl "alexGo-cloud/modules/{{.Module}}/controller/app"
)

func Register{{.ServiceName}}Routes(r *gin.RouterGroup, admin *adminctrl.Admin{{.ServiceName}}Controller, app *appctrl.App{{.ServiceName}}Controller) {
	adminGroup := r.Group("/admin/{{.Module}}")
	adminGroup.GET("/{{ plural .LowerName }}", admin.List)
	adminGroup.POST("/{{.LowerName}}", admin.Create)

	appGroup := r.Group("/app/{{.Module}}")
	appGroup.GET("/{{.LowerName}}/ping", app.Ping)
}

