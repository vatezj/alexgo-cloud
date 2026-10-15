package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/service"
	"alexGo-cloud/pkg/codegen/builder"
	cgmodel "alexGo-cloud/pkg/codegen/model"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// CodegenController codegen 配置 admin 接口（spec §4）。
// 信封统一走 pkg/resp：列表 Page、详情/变更/成功 OK、失败 Fail（码段推导 HTTP status）。
// 状态：校验/业务拒绝 → 400（1xxxx/3xxxx）、读库/系统失败 → 500（50000）——仓库无 404 惯例。
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
	g.GET("/db-tables", c.listDBTables)
	g.POST("/import", c.importTable)
	g.GET("/tables/:id", c.getTable)
	g.PUT("/tables/:id", c.updateTable)
	g.DELETE("/tables/:id", c.deleteTable)
	g.GET("/tables/:id/columns", c.listColumns)
	g.PUT("/tables/:id/columns", c.saveColumns)
	g.POST("/tables/:id/sync", c.sync)
	g.POST("/preview", c.preview)   // M3：纯内存渲染
	g.POST("/download", c.download) // M3：ZIP 流
}

func idParam(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(c, errs.New(errs.CodeParamInvalid, nil))
		return 0, false
	}
	return id, true
}

// svcFail 统一落 service 层错误（映射表）：不存在 → 30007、重复导入 → 30008、
// 校验类（sentinel / 显式字面量）→ 10001；其余裸透传 resp.Fail（非 BizError → 50000 + zap 日志）。
// service 层错误多为裸 error（service 不在本任务改动清单），语义按 errors.Is sentinel
// 与错误字面量判别——对齐 user/role 控制器「already exists」映射先例（Task 8）。
func svcFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, cgmodel.ErrTableNotFound):
		resp.Fail(c, errs.New(errs.CodeCodegenTableNotFound, nil))
	case strings.Contains(err.Error(), "already imported"):
		resp.Fail(c, errs.New(errs.CodeCodegenTableTaken, nil))
	case errors.Is(err, cgmodel.ErrTableInvalid), errors.Is(err, cgmodel.ErrColumnInvalid),
		strings.Contains(err.Error(), "invalid table name"),
		strings.Contains(err.Error(), "not in table"):
		resp.Fail(c, errs.New(errs.CodeParamInvalid, err))
	default:
		resp.Fail(c, err)
	}
}

func (c *CodegenController) listTables(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	items, total, err := c.svc.ListTables(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err) // 读库类 → 500 裸透传
		return
	}
	resp.Page(ctx, items, total, req)
}

// listDBTables 库内可导入表名（导入弹窗下拉）。读库类失败 → 500。
func (c *CodegenController) listDBTables(ctx *gin.Context) {
	names, err := c.svc.ListDBTables(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, names)
}

func (c *CodegenController) getTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	t, cols, err := c.svc.GetTable(ctx.Request.Context(), id)
	if err != nil {
		svcFail(ctx, err) // 不存在 → 30007（400）；读库失败裸透传 → 50000
		return
	}
	// 详情双字段（原顶层 {data,columns}）整体收进统一信封 data，键名不变。
	resp.OK(ctx, gin.H{"data": t, "columns": cols})
}

func (c *CodegenController) importTable(ctx *gin.Context) {
	var req struct {
		TableName string `json:"table_name" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	t, err := c.svc.ImportTable(ctx.Request.Context(), req.TableName)
	if err != nil {
		// 不存在/重复导入/校验 → 400 语义码；其余读库失败裸透传 → 50000
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, t)
}

func (c *CodegenController) updateTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var t model.CodegenTable
	if err := ctx.ShouldBindJSON(&t); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	t.ID = id // 路径 id 压过 body
	if err := c.svc.UpdateTable(ctx.Request.Context(), &t); err != nil {
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *CodegenController) deleteTable(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	if err := c.svc.DeleteTable(ctx.Request.Context(), id); err != nil {
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *CodegenController) listColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	cols, err := c.svc.ListColumns(ctx.Request.Context(), id)
	if err != nil {
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, cols)
}

func (c *CodegenController) saveColumns(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	var cols []*model.CodegenColumn
	if err := ctx.ShouldBindJSON(&cols); err != nil || len(cols) == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.svc.SaveColumns(ctx.Request.Context(), id, cols); err != nil {
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

func (c *CodegenController) sync(ctx *gin.Context) {
	id, ok := idParam(ctx)
	if !ok {
		return
	}
	res, err := c.svc.Sync(ctx.Request.Context(), id)
	if err != nil {
		svcFail(ctx, err)
		return
	}
	resp.OK(ctx, res)
}

// preview 渲染选中配置表（spec §4）。body: {"table_ids":[...]} → data:[{path,content}]。
// 纯内存：引擎失败只回 400/500，不落盘、不留半成品（spec §6）。
func (c *CodegenController) preview(ctx *gin.Context) {
	var req struct {
		TableIDs []uint64 `json:"table_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || len(req.TableIDs) == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	files, err := c.svc.Preview(ctx.Request.Context(), req.TableIDs)
	if err != nil {
		svcFail(ctx, err)
		return
	}
	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		items = append(items, gin.H{"path": f.Path, "content": string(f.Content)})
	}
	resp.OK(ctx, items)
}

// download 同一份渲染结果打成 ZIP 流（spec §4）。JSON 错误分支走统一信封，文件流分支不动。
func (c *CodegenController) download(ctx *gin.Context) {
	var req struct {
		TableIDs []uint64 `json:"table_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || len(req.TableIDs) == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	zipBytes, err := c.svc.Download(ctx.Request.Context(), req.TableIDs)
	if err != nil {
		svcFail(ctx, err)
		return
	}
	ctx.Header("Content-Disposition", `attachment; filename="codegen.zip"`)
	ctx.Data(200, "application/zip", zipBytes)
}
