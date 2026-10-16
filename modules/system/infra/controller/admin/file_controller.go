package admin

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/infra/model"
	"alexGo-cloud/modules/system/infra/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// FileController 统一文件端点：admin 受控面（JWT+权限码）+ public 免鉴权读。
// 免鉴权不免信封（spec §3）：JSON 分支全走 pkg/resp；流本体是唯一豁免。
type FileController struct {
	svc service.FileService
}

// NewFileController 构造文件控制器。
func NewFileController(svc service.FileService) *FileController {
	return &FileController{svc: svc}
}

// RegisterAdminRoutes 挂 /admin/infra/file（入参为 /api group 下的子组）。
func (c *FileController) RegisterAdminRoutes(g *gin.RouterGroup) {
	g.POST("/upload", c.upload)
	g.GET("/:id", c.downloadByID)
	g.DELETE("/:id", c.delete)
}

// RegisterPublicRoutes 挂 /public/infra/file——AuthMiddleware 只拦 /api/admin/**
// （pkg/middleware/auth.go），本组天然免登录，防枚举靠随机 public_key。
func (c *FileController) RegisterPublicRoutes(g *gin.RouterGroup) {
	g.GET("/:key", c.publicGet)
}

// upload multipart：字段 file + 表单域 public("true"/"false", 默认 false) + 可选 biz_type。
func (c *FileController) upload(ctx *gin.Context) {
	fh, err := ctx.FormFile("file")
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, err))
		return
	}
	src, err := fh.Open()
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	defer src.Close()

	meta, err := c.svc.Upload(ctx.Request.Context(), service.UploadInput{
		Name:    fh.Filename,
		Size:    fh.Size, // 仅预检；service 内按字节流限流兜底（Size 可伪造）
		Reader:  src,
		Public:  ctx.PostForm("public") == "true",
		BizType: ctx.PostForm("biz_type"),
	})
	if err != nil {
		resp.Fail(ctx, err) // service 已返 BizError；DB 裸错兜底 50000
		return
	}
	resp.OK(ctx, gin.H{
		"id":         meta.ID,
		"name":       meta.Name,
		"size":       meta.Size,
		"mime":       meta.Mime,
		"public_url": publicURL(meta),
	})
}

// downloadByID 受控下载（混合模型受控半边）：attachment 头 + 流式字节。
func (c *FileController) downloadByID(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	f, err := c.svc.GetByID(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	c.serve(ctx, f, "attachment")
}

// publicGet 按 public_key 读 Public=true 行；private/不存在统一 30010。
func (c *FileController) publicGet(ctx *gin.Context) {
	f, err := c.svc.GetByKey(ctx.Request.Context(), ctx.Param("key"))
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	c.serve(ctx, f, "inline")
}

// serve 流式输出（spec §5）：存库 mime、nosniff、RFC5987 文件名、缓存 1h（仅 public）。
func (c *FileController) serve(ctx *gin.Context, f *model.SystemFile, disposition string) {
	rc, err := c.svc.Open(ctx.Request.Context(), f)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	defer rc.Close()
	ctx.Header("Content-Type", f.Mime)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	if disposition == "inline" {
		ctx.Header("Cache-Control", "public, max-age=3600")
	}
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, rc) // 头已发，拷贝失败无法回信封——与 codegen download 同豁免
}

func (c *FileController) delete(ctx *gin.Context) {
	id, ok := pathID(ctx)
	if !ok {
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// pathID 解析 :id；非法/0 → 10001（信封），ok=false 表示已写响应。
// 与 codegen_controller.idParam 同款 strconv.ParseUint 惯例。
func pathID(ctx *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}

// publicURL public 行给对外链接；private 恒空串（前端不渲染死链）。
func publicURL(f *model.SystemFile) string {
	if !f.Public {
		return ""
	}
	return "/api/public/infra/file/" + f.PublicKey
}
