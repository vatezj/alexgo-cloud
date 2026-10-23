package admin

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// AdminController admin 端打牌记账监管（只读 + 强制关房；casbin 已鉴权）。
// 不取 claims：admin 路由由 casbin 中间件按 permissionRoutes 前缀放行，
// 业务侧只做参数与领域校验（本模块无「操作人」语义——监管动作不落操作者身份）。
type AdminController struct {
	rooms   service.RoomService
	records service.RecordService
	friends service.FriendService
}

func NewAdminController(rooms service.RoomService, records service.RecordService,
	friends service.FriendService) *AdminController {
	return &AdminController{rooms: rooms, records: records, friends: friends}
}

// ListRooms godoc
//
//	@Summary		房间分页
//	@Description	权限 scorecard:room；统一 pkg/resp 信封。status 缺省 -1 不过滤；room_code 空不过滤、非空按房号精确等值（spec §4.2）。
//	@Tags			scorecard-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Param			status	query	integer	false	"房态过滤，缺省 -1 不过滤"	Example(0)
//	@Param			room_code	query	string	false	"房号精确等值，空不过滤"	Example(PK888)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/scorecard/rooms [get]
func (c *AdminController) ListRooms(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		Status   *int   `form:"status"`
		RoomCode string `form:"room_code"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil { // PageReq binding max=100 挡超限
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	status := -1
	if req.Status != nil {
		status = *req.Status
	}
	list, total, err := c.rooms.AdminListRooms(ctx.Request.Context(), status, strings.TrimSpace(req.RoomCode), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// RoomDetail godoc
//
//	@Summary		房间详情（房行 + 成员 + 最近流水）
//	@Description	权限 scorecard:room；统一 pkg/resp 信封。房不存在/跨租户 30101。
//	@Tags			scorecard-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/scorecard/rooms/{id} [get]
func (c *AdminController) RoomDetail(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	d, err := c.rooms.AdminRoomDetail(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, d)
}

// CloseRoom godoc
//
//	@Summary		强制关房
//	@Description	权限 scorecard:room；监管强制关房，无房主校验。房不存在 30101、已关 30103。
//	@Tags			scorecard-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/scorecard/rooms/{id}/close [post]
func (c *AdminController) CloseRoom(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.rooms.AdminClose(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// ListRecords godoc
//
//	@Summary		战绩分页
//	@Description	权限 scorecard:record；统一 pkg/resp 信封。user_id/room_id 为 0 = 不过滤。
//	@Tags			scorecard-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Param			user_id	query	integer	false	"按玩家过滤，0 不过滤"	Example(1001)
//	@Param			room_id	query	integer	false	"按房间过滤，0 不过滤"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/scorecard/records [get]
func (c *AdminController) ListRecords(ctx *gin.Context) {
	var req struct {
		resp.PageReq
		UserID uint64 `form:"user_id"`
		RoomID uint64 `form:"room_id"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.records.AdminList(ctx.Request.Context(), req.UserID, req.RoomID, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// ListFriends godoc
//
//	@Summary		好友关系分页
//	@Description	权限 scorecard:friend；统一 pkg/resp 信封。全租户关系行（gorm 租户插件隔离），不做 userID 维度过滤。
//	@Tags			scorecard-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Success		200		{object}	map[string]interface{}
//	@Router			/admin/scorecard/friends [get]
func (c *AdminController) ListFriends(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.friends.AdminList(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req) // req 本身就是 PageReq（brief 原稿写 req.PageReq，编译期已校正）
}
