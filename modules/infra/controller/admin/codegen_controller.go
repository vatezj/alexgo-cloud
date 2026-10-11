package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/infra/model"
	"alexGo-cloud/modules/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
)

// CodegenController codegen 配置 admin 接口（spec §4）。
// 信封：列表 {data,total} / 详情 {data,columns} / 变更 {status:"ok"} / 错误 {error}。
// 状态码只用 400 与 500——仓库无 404 惯例。
type CodegenController struct {
	svc  service.CodegenService
	opts builder.Options
}

func NewCodegenController(svc service.CodegenService, opts builder.Options) *CodegenController {
	return &CodegenController{svc: svc, opts: opts}
}

// RegisterRoutes 挂在 server /api 组下，前缀 /admin/infra/codegen。
func (c *CodegenController) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/tables", c.listTables)
	g.POST("/import", c.importTable)
	g.GET("/tables/:id", c.getTable)
	g.PUT("/tables/:id", c.updateTable)
	g.DELETE("/tables/:id", c.deleteTable)
	g.GET("/tables/:id/columns", c.listColumns)
	g.PUT("/tables/:id/columns", c.saveColumns)
	g.POST("/tables/:id/sync", c.sync)
	g.GET("/tables/:id/generate", c.generate) // M3 占位
}

func idParam(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// fail 统一错误出口：读库类 → 500；校验/不存在/业务拒绝 → 400。
func fail(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

func (c *CodegenController) listTables(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "10"))
	items, total, err := c.svc.ListTables(ctx.Request.Context(), page, size)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

func (c *CodegenController) getTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	t, cols, err := c.svc.GetTable(ctx.Request.Context(), id)
	if err != nil {
		fail(ctx, http.StatusBadRequest, err) // 不存在 = 400
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": t, "columns": cols})
}

func (c *CodegenController) importTable(ctx *gin.Context) {
	var req struct {
		TableName string `json:"table_name" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	t, err := c.svc.ImportTable(ctx.Request.Context(), req.TableName)
	if err != nil {
		// 哨兵（不存在/主键规则）与重复导入 → 400；其余读库失败 → 500
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": t})
}

func (c *CodegenController) updateTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var t model.CodegenTable
	if err := ctx.ShouldBindJSON(&t); err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	t.ID = id // 路径 id 压过 body
	if err := c.svc.UpdateTable(ctx.Request.Context(), &t); err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) deleteTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	if err := c.svc.DeleteTable(ctx.Request.Context(), id); err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) listColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	cols, err := c.svc.ListColumns(ctx.Request.Context(), id)
	if err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": cols})
}

func (c *CodegenController) saveColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var cols []*model.CodegenColumn
	if err := ctx.ShouldBindJSON(&cols); err != nil || len(cols) == 0 {
		fail(ctx, http.StatusBadRequest, errOr(err, "empty columns"))
		return
	}
	if err := c.svc.SaveColumns(ctx.Request.Context(), id, cols); err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *CodegenController) sync(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	res, err := c.svc.Sync(ctx.Request.Context(), id)
	if err != nil {
		fail(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": res})
}

// generate 是 M3 能力（模板渲染+下载），M2 显式占位防误调。
func (c *CodegenController) generate(ctx *gin.Context) {
	fail(ctx, http.StatusInternalServerError, errM3Feature)
}

type featureNotReadyError struct{ msg string }

func (e *featureNotReadyError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "generate is an M3 feature"
}

var errM3Feature = &featureNotReadyError{}

func errOr(err error, fallback string) error {
	if err != nil {
		return err
	}
	return &featureNotReadyError{msg: fallback}
}
