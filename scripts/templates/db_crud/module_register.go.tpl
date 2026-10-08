package {{.Module}}

import (
	"github.com/gin-gonic/gin"

	adminctrl "alexGo-cloud/modules/{{.Module}}/controller/admin"
)

func Register{{.Entity}}DBRoutes(r *gin.RouterGroup, admin *adminctrl.Admin{{.Entity}}Controller) {
	g := r.Group("/admin/{{.Module}}")
	g.GET("/{{ lower .Plural }}", admin.List)
	g.POST("/{{ lower .Plural }}", admin.Create)
	g.DELETE("/{{ lower .Plural }}/:id", admin.Delete)
}

