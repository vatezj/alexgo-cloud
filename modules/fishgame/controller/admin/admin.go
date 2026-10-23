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

// ListFish godoc
//
//	@Summary		图鉴条目分页
//	@Description	权限 fishgame:fish；统一 pkg/resp 信封。rarity 空不过滤，status 缺省 -1 不过滤。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query		integer		false	"页码，默认 1"	Example(1)
//	@Param			size	query		integer		false	"每页条数，默认 10"	Example(10)
//	@Param			rarity	query		string		false	"normal|misc|rare|epic，空不过滤"	Example(rare)
//	@Param			status	query		integer		false	"0 停用 1 启用，缺省 -1 不过滤"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/fishes [get]
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

// CreateFish godoc
//
//	@Summary		新建图鉴条目
//	@Description	权限 fishgame:fish；fish_id 查重（30401）+ rarity/period/depth 枚举校验（10001）。
//	@Tags			fishgame-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body		service.FishInput		true	"图鉴条目"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/fishes [post]
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

// UpdateFish godoc
//
//	@Summary		更新图鉴条目（含启停）
//	@Description	权限 fishgame:fish；查无 → 30401；枚举校验同新建。
//	@Tags			fishgame-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id		path		integer		true	"行 id"
//	@Param			body	body		service.FishInput		true	"图鉴条目"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/fishes/{id} [put]
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

// DeleteFish godoc
//
//	@Summary		删除图鉴条目
//	@Description	权限 fishgame:fish；被 dex/事件引用 → 30402（改启停即可）；查无 → 30401。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path		integer		true	"行 id"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/fishgame/fishes/{id} [delete]
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

// ListConfig godoc
//
//	@Summary		运营覆盖分页
//	@Description	权限 fishgame:config；type 空不过滤（luck|fish_king）。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query		integer		false	"页码"	Example(1)
//	@Param			size	query		integer		false	"每页条数"	Example(10)
//	@Param			type	query		string		false	"luck|fish_king，空不过滤"	Example(fish_king)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/configs [get]
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

// CreateConfig godoc
//
//	@Summary		新建运营覆盖
//	@Description	权限 fishgame:config；luck 校验 period/depth_bias≤0.3；fish_king 的 fish_id 必须表内启用（30403）且 week 必填。
//	@Tags			fishgame-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body		service.ConfigInput		true	"运营覆盖"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/configs [post]
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

// UpdateConfig godoc
//
//	@Summary		更新运营覆盖（含启停）
//	@Description	权限 fishgame:config；查无 → 30404；校验同新建。
//	@Tags			fishgame-admin
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id		path		integer		true	"行 id"
//	@Param			body	body		service.ConfigInput		true	"运营覆盖"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/configs/{id} [put]
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

// DeleteConfig godoc
//
//	@Summary		删除运营覆盖
//	@Description	权限 fishgame:config；查无 → 30404。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path		integer		true	"行 id"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/fishgame/configs/{id} [delete]
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

// ListEvents godoc
//
//	@Summary		埋点事件分页
//	@Description	权限 fishgame:event；name/player_id/起止时间（客户端本地 epoch 秒）筛选，id 倒序。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page		query		integer		false	"页码"	Example(1)
//	@Param			size		query		integer		false	"每页条数"	Example(10)
//	@Param			name		query		string		false	"事件名，如 catch/sign_in"	Example(catch)
//	@Param			player_id	query		integer		false	"玩家行 id"	Example(1)
//	@Param			start_t		query		integer		false	"起（含）unix 秒"	Example(1760600000)
//	@Param			end_t		query		integer		false	"止（含）unix 秒"	Example(1761200000)
//	@Success		200			{object}	map[string]interface{}
//	@Router			/admin/fishgame/events [get]
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

// ListPlayers godoc
//
//	@Summary		玩家分页
//	@Description	权限 fishgame:player；openid/nick 模糊 + 行内点亮数（只聚合当页）。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query		integer		false	"页码"	Example(1)
//	@Param			size	query		integer		false	"每页条数"	Example(10)
//	@Param			keyword	query		string		false	"openid/nick 模糊"	Example(阿)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/fishgame/players [get]
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

// PlayerDetail godoc
//
//	@Summary		玩家详情
//	@Description	权限 fishgame:player；基本 + 点亮明细（count≥1）+ 最近 20 事件；查无 → 30002。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path		integer		true	"玩家行 id"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/fishgame/players/{id} [get]
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

// PlayerRank godoc
//
//	@Summary		玩家榜单位次
//	@Description	权限 fishgame:player；复用好友榜双口径找本人位置（客服排查用）。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path		integer		true	"玩家行 id"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/fishgame/players/{id}/rank [get]
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

// Stats godoc
//
//	@Summary		运营看板聚合（近 7 日）
//	@Description	权限 fishgame:stats；玩家总量/今日新增、事件计数、按日趋势、钓鱼漏斗（catch/empty/escape/broken）、签到/茶篓/广告计数。
//	@Tags			fishgame-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	service.StatsOut
//	@Router			/admin/fishgame/stats [get]
func (c *AdminController) Stats(ctx *gin.Context) {
	st, err := c.svc.Stats(ctx.Request.Context(), time.Now())
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, st)
}
