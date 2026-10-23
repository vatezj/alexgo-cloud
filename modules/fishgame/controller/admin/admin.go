package admin

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// AdminController admin 端 fishgame 五组端点（casbin 已按 permissionRoutes 前缀鉴权，
// 照 scorecard 模式不取 claims；业务侧只做参数与领域校验）。
type AdminController struct {
	svc *service.AdminService
}

func NewAdminController(svc *service.AdminService) *AdminController {
	return &AdminController{svc: svc}
}

// paramID 解析 :id 路径参数（0/非法 → 10001）。
func paramID(ctx *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}

// ---- fish ----

// ListFish GET /fishes：rarity 空不过滤，status 缺省 -1 不过（spec §4.2）。
func (c *AdminController) ListFish(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Rarity string `form:"rarity"`
		Status *int   `form:"status"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	status := -1
	if req.Status != nil {
		status = *req.Status
	}
	list, total, err := c.svc.ListFish(ctx.Request.Context(), req.Rarity, status, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// CreateFish POST /fishes：fish_id 查重（30401）+ 枚举校验（10001）。
func (c *AdminController) CreateFish(ctx *gin.Context) {
	var in service.FishInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	f, err := c.svc.CreateFish(ctx.Request.Context(), in)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, f)
}

// UpdateFish PUT /fishes/:id：更新（含 status 启停）。
func (c *AdminController) UpdateFish(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	var in service.FishInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	f, err := c.svc.UpdateFish(ctx.Request.Context(), id, in)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, f)
}

// DeleteFish DELETE /fishes/:id：引用检查 → 30402。
func (c *AdminController) DeleteFish(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	if err := c.svc.DeleteFish(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ---- config ----

// ListConfig GET /configs：type 空不过滤（luck|fish_king）。
func (c *AdminController) ListConfig(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Type string `form:"type"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.svc.ListConfig(ctx.Request.Context(), req.Type, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// CreateConfig POST /configs：fish_king 的 fish_id 校验 → 30403。
func (c *AdminController) CreateConfig(ctx *gin.Context) {
	var in service.ConfigInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	cfg, err := c.svc.CreateConfig(ctx.Request.Context(), in)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, cfg)
}

// UpdateConfig PUT /configs/:id：查无 → 30404。
func (c *AdminController) UpdateConfig(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	var in service.ConfigInput
	if err := ctx.ShouldBindJSON(&in); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	cfg, err := c.svc.UpdateConfig(ctx.Request.Context(), id, in)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, cfg)
}

// DeleteConfig DELETE /configs/:id：查无 → 30404。
func (c *AdminController) DeleteConfig(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	if err := c.svc.DeleteConfig(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ---- events ----

// ListEvents GET /events：name/player_id/起止时间筛选，id 倒序（repo 保证）。
func (c *AdminController) ListEvents(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Name     string `form:"name"`
		PlayerID uint64 `form:"player_id"`
		StartT   int64  `form:"start_t"`
		EndT     int64  `form:"end_t"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.svc.ListEvents(ctx.Request.Context(), req.Name, req.PlayerID,
		req.StartT, req.EndT, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// ---- players ----

// ListPlayers GET /players：openid/nick 模糊 + 行内点亮数。
func (c *AdminController) ListPlayers(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Keyword string `form:"keyword"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.svc.ListPlayers(ctx.Request.Context(), req.Keyword, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// PlayerDetail GET /players/:id：基本 + 点亮明细 + 最近 20 事件。
func (c *AdminController) PlayerDetail(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	d, err := c.svc.PlayerDetail(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, d)
}

// PlayerRank GET /players/:id/rank：当前榜口径次位（客服排查）。
func (c *AdminController) PlayerRank(ctx *gin.Context) {
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	pos, err := c.svc.PlayerRank(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, pos)
}

// ---- stats ----

// Stats GET /stats：近 7 日看板聚合（spec §4.9）。
func (c *AdminController) Stats(ctx *gin.Context) {
	st, err := c.svc.Stats(ctx.Request.Context(), time.Now())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, st)
}
