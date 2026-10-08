package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/{{.Module}}/model"
	"alexGo-cloud/modules/{{.Module}}/service"
)

type Admin{{.ServiceName}}Controller struct {
	svc service.{{.ServiceName}}Service
}

func NewAdmin{{.ServiceName}}Controller(svc service.{{.ServiceName}}Service) *Admin{{.ServiceName}}Controller {
	return &Admin{{.ServiceName}}Controller{svc: svc}
}

func (c *Admin{{.ServiceName}}Controller) List(ctx *gin.Context) {
	list, err := c.svc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

type create{{.ServiceName}}Request struct {
{{- range .Fields }}
	{{.GoName}} {{.GoType}} `json:"{{.JSONName}}"`
{{- end }}
}

func (c *Admin{{.ServiceName}}Controller) Create(ctx *gin.Context) {
	var req create{{.ServiceName}}Request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entity := &model.{{.ServiceName}}{
{{- range .Fields }}
		{{.GoName}}: req.{{.GoName}},
{{- end }}
	}
	if err := c.svc.Create(ctx.Request.Context(), entity); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
