package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

type BasicController struct {
	deptSvc   service.DeptService
	postSvc   service.PostService
	dictSvc   service.DictService
	configSvc service.ConfigService
	noticeSvc service.NoticeService
}

func NewBasicController(
	deptSvc service.DeptService,
	postSvc service.PostService,
	dictSvc service.DictService,
	configSvc service.ConfigService,
	noticeSvc service.NoticeService,
) *BasicController {
	return &BasicController{
		deptSvc:   deptSvc,
		postSvc:   postSvc,
		dictSvc:   dictSvc,
		configSvc: configSvc,
		noticeSvc: noticeSvc,
	}
}

// claimsOf 取中间件注入的 claims（生产路径由 auth 中间件保证；handler 内再判一次作纵深防御，
// 与 controller/vben 的同名 helper 同构）。
func claimsOf(ctx *gin.Context) (*auth.Claims, bool) {
	v, exists := ctx.Get("claims")
	if !exists {
		return nil, false
	}
	cl, ok := v.(*auth.Claims)
	if !ok || cl == nil {
		return nil, false
	}
	return cl, true
}

func (c *BasicController) ListDepts(ctx *gin.Context) {
	list, err := c.deptSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreateDept(ctx *gin.Context) {
	var req model.Dept
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.deptSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeleteDept(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.deptSvc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdateDept(ctx *gin.Context) {
	var req model.Dept
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.deptSvc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) ListPosts(ctx *gin.Context) {
	list, err := c.postSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreatePost(ctx *gin.Context) {
	var req model.Post
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.postSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeletePost(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.postSvc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdatePost(ctx *gin.Context) {
	var req model.Post
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.postSvc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) ListDictTypes(ctx *gin.Context) {
	list, err := c.dictSvc.ListTypes(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreateDictType(ctx *gin.Context) {
	var req model.DictType
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.dictSvc.CreateType(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeleteDictType(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.dictSvc.DeleteType(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdateDictType(ctx *gin.Context) {
	var req model.DictType
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.dictSvc.UpdateType(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) ListDictDatas(ctx *gin.Context) {
	typeID, _ := strconv.ParseUint(ctx.Query("type_id"), 10, 64)
	if typeID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid type_id"})
		return
	}
	list, err := c.dictSvc.ListDatas(ctx.Request.Context(), typeID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreateDictData(ctx *gin.Context) {
	var req model.DictData
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.dictSvc.CreateData(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeleteDictData(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.dictSvc.DeleteData(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdateDictData(ctx *gin.Context) {
	var req model.DictData
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.dictSvc.UpdateData(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) ListConfigs(ctx *gin.Context) {
	list, err := c.configSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreateConfig(ctx *gin.Context) {
	var req model.SystemConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.configSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeleteConfig(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.configSvc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdateConfig(ctx *gin.Context) {
	var req model.SystemConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.configSvc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) UpdateNotice(ctx *gin.Context) {
	var req model.SystemNotice
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.noticeSvc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *BasicController) ListNotices(ctx *gin.Context) {
	list, err := c.noticeSvc.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

func (c *BasicController) CreateNotice(ctx *gin.Context) {
	var req model.SystemNotice
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := c.noticeSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": out})
}

func (c *BasicController) DeleteNotice(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.noticeSvc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListMyNotices 我的通知：本租户公告 + 当前用户已读标记（P0-1）。
func (c *BasicController) ListMyNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	list, err := c.noticeSvc.ListMine(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

// MarkNoticeRead 标记单条已读（幂等；公告不存在或跨租户 → 400）。
func (c *BasicController) MarkNoticeRead(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.noticeSvc.MarkRead(ctx.Request.Context(), claims.UserID, id); err != nil {
		if errors.Is(err, service.ErrNoticeNotFound) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ReadAllNotices 一键全部标记已读（幂等）。
func (c *BasicController) ReadAllNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := c.noticeSvc.MarkAllRead(ctx.Request.Context(), claims.UserID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
