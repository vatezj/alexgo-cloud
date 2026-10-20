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

// ListRooms GET /admin/scorecard/rooms：房间分页，status 缺省 -1 不过滤；
// room_code 缺省空串 = 不过滤，非空按房号精确等值（spec §4.2 房间列表筛选）。
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

// RoomDetail GET /admin/scorecard/rooms/:id：房行 + 成员 + 最近流水（30101 房不存在）。
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

// CloseRoom POST /admin/scorecard/rooms/:id/close：强制关房（无房主校验，已关 30103）。
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

// ListRecords GET /admin/scorecard/records：战绩分页，user_id/room_id 0 = 不过滤。
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

// ListFriends GET /admin/scorecard/friends：好友关系分页。
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
