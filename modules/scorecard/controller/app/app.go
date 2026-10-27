package app

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/service"
	"alexGo-cloud/modules/system/oauth2/social"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// AppController app 端打牌记账接口（Bearer 自鉴权，不走 casbin）。
type AppController struct {
	rooms     service.RoomService
	records   service.RecordService
	friends   service.FriendService
	validator token.Validator
	// Task 2 小程序码：渠道配置取数 + 微信取码（social 窄接口，scorecard module 注入）。
	apps social.AppGetter
	wx   social.Wxacoder
	// wxMu/wxCache 小程序码进程内按房号缓存（5min，见 wxacode.go）。
	wxMu    sync.Mutex
	wxCache map[string]wxacodeCacheEntry
	// wxEnvVersion 小程序码 env_version（构造时从全局配置解析，见 wxEnvVersion）。
	wxEnvVersion string
}

func NewAppController(rooms service.RoomService, records service.RecordService,
	friends service.FriendService, validator token.Validator,
	apps social.AppGetter, wx social.Wxacoder, cfg *config.Config) *AppController {
	return &AppController{
		rooms: rooms, records: records, friends: friends, validator: validator,
		apps: apps, wx: wx,
		wxCache:       make(map[string]wxacodeCacheEntry),
		wxEnvVersion: wxEnvVersion(cfg),
	}
}

// claims 解析 Bearer 并校验 member 身份；成功后把 token 内租户写回请求 ctx
// （spec §4.3，与 pkg/middleware/auth.go admin 路径同一 API）——TenantMiddleware
// 按 X-Tenant-ID 头解析的值在此被覆盖，伪造头无法跨租户；失败已写响应，ok=false 直接 return。
// 全部 handler 均先过本函数再取 ctx.Request.Context()，repo 层因此恒拿 token 租户。
func (c *AppController) claims(ctx *gin.Context) (*token.Claims, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	cl, err := c.validator.Validate(ctx.Request.Context(), raw)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	if cl.UserType != token.UserTypeMember {
		resp.Fail(ctx, errs.New(errs.CodeForbidden, nil))
		return nil, false
	}
	ctx.Request = ctx.Request.WithContext(tenant.WithTenantID(ctx.Request.Context(), cl.TenantID))
	return cl, true
}

func pathID(ctx *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(ctx.Param(name), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return 0, false
	}
	return id, true
}

// CreateRoom godoc
//
//	@Summary		创建房间
//	@Description	创建入参见 service.CreateRoomReq；统一 pkg/resp 信封。room_code 可空=服务端生成，指定被占 30106；data 为最新房间快照。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	service.CreateRoomReq	true	"建房参数"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms [post]
func (c *AppController) CreateRoom(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req service.CreateRoomReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.Create(ctx.Request.Context(), cl.UserID, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

// JoinRoom godoc
//
//	@Summary		按房号加入房间
//	@Description	统一 pkg/resp 信封，data 为最新房间快照。房不存在 30101、已关闭 30103、已结算 30104、任一方向被拉黑 30108。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"房号"	Example({"room_code": "PK888"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/join [post]
func (c *AppController) JoinRoom(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		RoomCode string `json:"room_code"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RoomCode == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.Join(ctx.Request.Context(), cl.UserID, req.RoomCode)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

// Snapshot godoc
//
//	@Summary		房间快照
//	@Description	房行 + 成员 + 累计；统一 pkg/resp 信封。房不存在/跨租户 30101、非成员 30102（防外人窥房）。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/snapshot [get]
func (c *AppController) Snapshot(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	snap, err := c.rooms.Snapshot(ctx.Request.Context(), cl.UserID, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

// UpdateConfig godoc
//
//	@Summary		更新房间配置（仅房主）
//	@Description	指针字段只改提供的项，数值项「从下把生效」写 pending_*（spec §3.2）。仅房主 30105；房不存在 30101、已结算 30104、已关闭 30103。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Param			body	body	service.ConfigReq	true	"改配置（缺省项不改）"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/config [put]
func (c *AppController) UpdateConfig(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req service.ConfigReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	snap, err := c.rooms.UpdateConfig(ctx.Request.Context(), cl.UserID, id, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, snap)
}

// Expense godoc
//
//	@Summary		记一笔分支出
//	@Description	入参见 service.ExpenseReq：score>0、to_user_id≠0 且≠自己、client_request_id 必填（幂等键，重放回放原结果）；茶饭单笔 ≤1000000。房不存在 30101、已结算 30104、已关闭 30103、非成员 30102、收/付方不在房 30113。data 含最新快照 + 流水号。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Param			body	body	service.ExpenseReq	true	"记账入参"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/expense [post]
func (c *AppController) Expense(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req service.ExpenseReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	res, err := c.rooms.Expense(ctx.Request.Context(), cl.UserID, id, req)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, res)
}

// Logs godoc
//
//	@Summary		房间流水分页
//	@Description	统一 pkg/resp 信封（list/total/page/size）。房不存在 30101、非成员 30102。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/logs [get]
func (c *AppController) Logs(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil { // page/size；PageReq binding 的 max=100 挡超限
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults() // page=1 size=10（无返回值方法，绑定后再调）
	list, total, err := c.rooms.Logs(ctx.Request.Context(), cl.UserID, id, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

// Settle godoc
//
//	@Summary		结算牌局（仅房主）
//	@Description	统一 pkg/resp 信封。房不存在 30101、非房主 30105、已结算 30104、已关闭 30103。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/settle [post]
func (c *AppController) Settle(ctx *gin.Context) { c.roomAction(ctx, c.rooms.Settle) }

// Leave godoc
//
//	@Summary		成员离房
//	@Description	统一 pkg/resp 信封。房不存在 30101、非成员 30102。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/leave [post]
func (c *AppController) Leave(ctx *gin.Context) { c.roomAction(ctx, c.rooms.Leave) }

// roomAction userID+roomID 双参动作的统一壳（Settle/Leave）。
func (c *AppController) roomAction(ctx *gin.Context, fn func(ctx context.Context, userID, roomID uint64) error) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	if err := fn(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// TransferOwner godoc
//
//	@Summary		转让房主（仅房主）
//	@Description	统一 pkg/resp 信封。同人转让 30107、目标不在房 30113；房不存在 30101、非房主 30105、已结算 30104、已关闭 30103。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			id	path	integer	true	"房间 ID"	Example(1)
//	@Param			body	body	object	true	"转让目标"	Example({"target_user_id": 1002})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/rooms/{id}/transfer-owner [post]
func (c *AppController) TransferOwner(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "id")
	if !ok {
		return
	}
	var req struct {
		TargetUserID uint64 `json:"target_user_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.TargetUserID == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.rooms.TransferOwner(ctx.Request.Context(), cl.UserID, id, req.TargetUserID); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// MyRecords godoc
//
//	@Summary		我的战绩分页
//	@Description	仅本人行，统一 pkg/resp 信封（list/total/page/size）。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			page	query	integer	false	"页码，默认 1"	Example(1)
//	@Param			size	query	integer	false	"每页条数，默认 10，≤100"	Example(10)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/records [get]
func (c *AppController) MyRecords(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.records.MyRecords(ctx.Request.Context(), cl.UserID, req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req)
}

// RecordDetail godoc
//
//	@Summary		战绩详情（全房流水行）
//	@Description	统一 pkg/resp 信封。记录不存在 30110、非本人记录 30111。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			recordId	path	integer	true	"战绩 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/records/{recordId} [get]
func (c *AppController) RecordDetail(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "recordId")
	if !ok {
		return
	}
	rows, err := c.records.Detail(ctx.Request.Context(), cl.UserID, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, rows)
}

// DeleteRecord godoc
//
//	@Summary		删除我的战绩
//	@Description	统一 pkg/resp 信封。记录不存在 30110、非本人记录 30111（仅本人可删）。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			recordId	path	integer	true	"战绩 ID"	Example(1)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/records/{recordId} [delete]
func (c *AppController) DeleteRecord(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "recordId")
	if !ok {
		return
	}
	if err := c.records.Delete(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// Stat godoc
//
//	@Summary		我的战绩统计
//	@Description	统一 pkg/resp 信封，data = {total_score, win, lose, draw, win_rate}（分母 0 → 0）。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/stat [get]
func (c *AppController) Stat(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	st, err := c.records.Stat(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, st)
}

// ListFriends godoc
//
//	@Summary		我的好友列表
//	@Description	统一 pkg/resp 信封，data = FriendItem 数组直出（user_id/nickname/avatar/rank_enabled/total_score/win_rate/is_blocked），非分页。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/friends [get]
func (c *AppController) ListFriends(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	items, err := c.friends.List(ctx.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, items)
}

// AddFriend godoc
//
//	@Summary		添加好友
//	@Description	双向各写一行单事务；已是好友 30109、任一方向被拉黑 30108、目标不存在 30002、自加/0 10001。统一 pkg/resp 信封。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			body	body	object	true	"好友用户 ID"	Example({"friend_user_id": 1002})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/friends [post]
func (c *AppController) AddFriend(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		FriendUserID uint64 `json:"friend_user_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.FriendUserID == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.Add(ctx.Request.Context(), cl.UserID, req.FriendUserID); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// RemoveFriend godoc
//
//	@Summary		删除好友
//	@Description	双向删行单事务，幂等（无关系也成功）；自删/0 10001。统一 pkg/resp 信封。
//	@Tags			scorecard-app
//	@Produce		json
//	@Security		Bearer
//	@Param			userId	path	integer	true	"好友用户 ID"	Example(1002)
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/friends/{userId} [delete]
func (c *AppController) RemoveFriend(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	if err := c.friends.Remove(ctx.Request.Context(), cl.UserID, id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// SetFriendBlack godoc
//
//	@Summary		拉黑 / 解除拉黑
//	@Description	blocked=1 幂等拉黑（已存在直接成功），0 幂等解除（不存在也是成功）；仅接受 0/1，自拉黑 10001。统一 pkg/resp 信封。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			userId	path	integer	true	"目标用户 ID"	Example(1002)
//	@Param			body	body	object	true	"0 解除 1 拉黑"	Example({"blocked": 1})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/friends/{userId}/black [post]
func (c *AppController) SetFriendBlack(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	var req struct {
		Blocked int `json:"blocked"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || (req.Blocked != 0 && req.Blocked != 1) {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.SetBlack(ctx.Request.Context(), cl.UserID, id, req.Blocked == 1); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// SetFriendRank godoc
//
//	@Summary		好友排名参与开关
//	@Description	enabled=1 参与 / 0 退出，只更新我方单行，行不存在空更新（幂等）；仅接受 0/1，10001 校验失败。统一 pkg/resp 信封。
//	@Tags			scorecard-app
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Param			userId	path	integer	true	"好友用户 ID"	Example(1002)
//	@Param			body	body	object	true	"0 退出 1 参与"	Example({"enabled": 1})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/scorecard/friends/{userId}/rank [put]
func (c *AppController) SetFriendRank(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	id, ok := pathID(ctx, "userId")
	if !ok {
		return
	}
	var req struct {
		Enabled int `json:"enabled"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || (req.Enabled != 0 && req.Enabled != 1) {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := c.friends.SetRank(ctx.Request.Context(), cl.UserID, id, req.Enabled == 1); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
