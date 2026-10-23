package admin

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
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

// ListDepts godoc
//
//	@Summary		部门列表（全量树）
//	@Description	权限 system:dept；当前租户全量部门树，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/depts [get]
func (c *BasicController) ListDepts(ctx *gin.Context) {
	list, err := c.deptSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreateDept godoc
//
//	@Summary		新建部门
//	@Description	权限 system:dept。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.Dept	true	"部门"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/depts [post]
func (c *BasicController) CreateDept(ctx *gin.Context) {
	var req model.Dept
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.deptSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeleteDept godoc
//
//	@Summary		删除部门
//	@Description	权限 system:dept。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"部门 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/depts/{id} [delete]
func (c *BasicController) DeleteDept(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.deptSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdateDept godoc
//
//	@Summary		更新部门
//	@Description	权限 system:dept；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.Dept	true	"部门（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/depts [put]
func (c *BasicController) UpdateDept(ctx *gin.Context) {
	var req model.Dept
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.deptSvc.Update(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListPosts godoc
//
//	@Summary		岗位列表（全量）
//	@Description	权限 system:post；当前租户全量岗位，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/posts [get]
func (c *BasicController) ListPosts(ctx *gin.Context) {
	list, err := c.postSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreatePost godoc
//
//	@Summary		新建岗位
//	@Description	权限 system:post。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.Post	true	"岗位"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/posts [post]
func (c *BasicController) CreatePost(ctx *gin.Context) {
	var req model.Post
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.postSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeletePost godoc
//
//	@Summary		删除岗位
//	@Description	权限 system:post。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"岗位 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/posts/{id} [delete]
func (c *BasicController) DeletePost(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.postSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdatePost godoc
//
//	@Summary		更新岗位
//	@Description	权限 system:post；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.Post	true	"岗位（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/posts [put]
func (c *BasicController) UpdatePost(ctx *gin.Context) {
	var req model.Post
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.postSvc.Update(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListDictTypes godoc
//
//	@Summary		字典类型列表（全量）
//	@Description	权限 system:dict；当前租户全量字典类型，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/dict/types [get]
func (c *BasicController) ListDictTypes(ctx *gin.Context) {
	list, err := c.dictSvc.ListTypes(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreateDictType godoc
//
//	@Summary		新建字典类型
//	@Description	权限 system:dict。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.DictType	true	"字典类型"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/dict/types [post]
func (c *BasicController) CreateDictType(ctx *gin.Context) {
	var req model.DictType
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.dictSvc.CreateType(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeleteDictType godoc
//
//	@Summary		删除字典类型
//	@Description	权限 system:dict。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"字典类型 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/dict/types/{id} [delete]
func (c *BasicController) DeleteDictType(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.dictSvc.DeleteType(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdateDictType godoc
//
//	@Summary		更新字典类型
//	@Description	权限 system:dict；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.DictType	true	"字典类型（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/dict/types [put]
func (c *BasicController) UpdateDictType(ctx *gin.Context) {
	var req model.DictType
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.dictSvc.UpdateType(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListDictDatas godoc
//
//	@Summary		字典数据列表（按类型）
//	@Description	权限 system:dict；type_id 必填，缺失/为 0 → 10001。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			type_id	query	integer	true	"字典类型 ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/dict/datas [get]
func (c *BasicController) ListDictDatas(ctx *gin.Context) {
	typeID, _ := strconv.ParseUint(ctx.Query("type_id"), 10, 64)
	if typeID == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	list, err := c.dictSvc.ListDatas(ctx.Request.Context(), typeID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreateDictData godoc
//
//	@Summary		新建字典数据
//	@Description	权限 system:dict。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.DictData	true	"字典数据"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/dict/datas [post]
func (c *BasicController) CreateDictData(ctx *gin.Context) {
	var req model.DictData
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.dictSvc.CreateData(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeleteDictData godoc
//
//	@Summary		删除字典数据
//	@Description	权限 system:dict。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"字典数据 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/dict/datas/{id} [delete]
func (c *BasicController) DeleteDictData(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.dictSvc.DeleteData(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdateDictData godoc
//
//	@Summary		更新字典数据
//	@Description	权限 system:dict；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.DictData	true	"字典数据（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/dict/datas [put]
func (c *BasicController) UpdateDictData(ctx *gin.Context) {
	var req model.DictData
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.dictSvc.UpdateData(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListConfigs godoc
//
//	@Summary		系统配置列表（全量）
//	@Description	权限 system:config；当前租户全量配置，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/configs [get]
func (c *BasicController) ListConfigs(ctx *gin.Context) {
	list, err := c.configSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreateConfig godoc
//
//	@Summary		新建系统配置
//	@Description	权限 system:config。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.SystemConfig	true	"系统配置"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/configs [post]
func (c *BasicController) CreateConfig(ctx *gin.Context) {
	var req model.SystemConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.configSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeleteConfig godoc
//
//	@Summary		删除系统配置
//	@Description	权限 system:config。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"配置 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/configs/{id} [delete]
func (c *BasicController) DeleteConfig(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.configSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdateConfig godoc
//
//	@Summary		更新系统配置
//	@Description	权限 system:config；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.SystemConfig	true	"系统配置（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/configs [put]
func (c *BasicController) UpdateConfig(ctx *gin.Context) {
	var req model.SystemConfig
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.configSvc.Update(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UpdateNotice godoc
//
//	@Summary		更新公告
//	@Description	权限 system:notice；无 :id 路径参数，目标 ID 在 body 内。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.SystemNotice	true	"公告（含 ID）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/notices [put]
func (c *BasicController) UpdateNotice(ctx *gin.Context) {
	var req model.SystemNotice
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.noticeSvc.Update(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListNotices godoc
//
//	@Summary		公告列表（全量）
//	@Description	权限 system:notice；当前租户全量公告，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/notices [get]
func (c *BasicController) ListNotices(ctx *gin.Context) {
	list, err := c.noticeSvc.List(ctx.Request.Context())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// CreateNotice godoc
//
//	@Summary		新建公告
//	@Description	权限 system:notice。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	model.SystemNotice	true	"公告"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/admin/system/notices [post]
func (c *BasicController) CreateNotice(ctx *gin.Context) {
	var req model.SystemNotice
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	out, err := c.noticeSvc.Create(ctx.Request.Context(), &req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, out)
}

// DeleteNotice godoc
//
//	@Summary		删除公告
//	@Description	权限 system:notice。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"公告 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/notices/{id} [delete]
func (c *BasicController) DeleteNotice(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.noticeSvc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListMyNotices godoc
//
//	@Summary		我的通知（本租户公告 + 已读标记）
//	@Description	权限 system:notice；P0-1 我的通知：本租户公告附带当前用户已读标记。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Router			/admin/system/notices/mine [get]
func (c *BasicController) ListMyNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	list, err := c.noticeSvc.ListMine(ctx.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, list)
}

// MarkNoticeRead godoc
//
//	@Summary		标记单条通知已读
//	@Description	权限 system:notice；幂等。公告不存在或跨租户 → CodeNoticeNotFound 400。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"公告 ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/admin/system/notices/{id}/read [post]
func (c *BasicController) MarkNoticeRead(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.noticeSvc.MarkRead(ctx.Request.Context(), claims.UserID, id); err != nil {
		if errors.Is(err, service.ErrNoticeNotFound) {
			resp.Fail(ctx, errs.New(errs.CodeNoticeNotFound, nil))
			return
		}
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ReadAllNotices godoc
//
//	@Summary		一键全部标记已读
//	@Description	权限 system:notice；幂等，无请求体。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Router			/admin/system/notices/read-all [post]
func (c *BasicController) ReadAllNotices(ctx *gin.Context) {
	claims, ok := claimsOf(ctx)
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	if err := c.noticeSvc.MarkAllRead(ctx.Request.Context(), claims.UserID); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
