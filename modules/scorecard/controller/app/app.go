package app

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/scorecard/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/token"
)

// AppController app 端打牌记账接口（Bearer 自鉴权，不走 casbin）。
type AppController struct {
	rooms     service.RoomService
	records   service.RecordService
	friends   service.FriendService
	validator token.Validator
}

func NewAppController(rooms service.RoomService, records service.RecordService,
	friends service.FriendService, validator token.Validator) *AppController {
	return &AppController{rooms: rooms, records: records, friends: friends, validator: validator}
}

// claims 解析 Bearer 并校验 member 身份；失败已写响应，ok=false 直接 return。
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

func (c *AppController) Settle(ctx *gin.Context) { c.roomAction(ctx, c.rooms.Settle) }
func (c *AppController) Leave(ctx *gin.Context)  { c.roomAction(ctx, c.rooms.Leave) }

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
