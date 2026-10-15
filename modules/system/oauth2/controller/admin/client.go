package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/oauth2/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// ClientController OAuth2 client 管理（统一信封 pkg/resp；非 RFC 6749 信封——
// 协议信封只属于 /api/oauth2/*，见 protocol.go 禁改边界）。
type ClientController struct {
	svc *oauth2svc.ClientService
}

func NewClientController(svc *oauth2svc.ClientService) *ClientController {
	return &ClientController{svc: svc}
}

// createClientResponse Create 响应：嵌入 model 展平字段，外层 ClientSecret
// （深度 0）覆盖嵌入的 json:"-"（深度 1）——明文仅此一次回显（裁决 2，spec §3.1）。
type createClientResponse struct {
	*model.Oauth2Client
	ClientSecret string `json:"client_secret"`
}

// createClientRequest Create 请求体：model.ClientSecret 带 json:"-"，encoding/json
// 绑定时会整体跳过该字段——明文 secret 须由外层同名字段（深度 0）单独收取，
// 绑定后再写回 model 交给 svc.Create 原地哈希（否则 body 里的 secret 被静默丢弃，
// 落库成无 secret 的公开客户端，创建响应也无从回显——裁决 2 的前置）。
type createClientRequest struct {
	model.Oauth2Client
	ClientSecret string `json:"client_secret"`
}

// List GET /admin/oauth2/clients?page&size → data:{list,total,page,size}（resp.Page）。
// 分页入参统一 resp.PageReq：binding max=100 取代原手写 size≤200 截断（裁决 4）——
// 超限从静默截断变 400；前端 `size=200` 拉满的旧用法在 Task 11 改真分页前会被拦
// （plan 预期窗口，Task 9 codegen size=200 同款）。
func (c *ClientController) List(ctx *gin.Context) {
	var req resp.PageReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	items, total, err := c.svc.List(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err) // 读库类 → 500 裸透传
		return
	}
	resp.Page(ctx, items, total, req)
}

// Create POST /admin/oauth2/clients → data 回显；client_id 空先行 400（裁决 3）。
func (c *ClientController) Create(ctx *gin.Context) {
	var req createClientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.ClientID == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	req.ID = 0                            // 新建忽略请求 id
	plain := req.ClientSecret             // 明文只在此刻存在（裁决 2）
	req.Oauth2Client.ClientSecret = plain // json:"-" 不参与绑定，手动写回再哈希
	if err := c.svc.Create(ctx.Request.Context(), &req.Oauth2Client); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, createClientResponse{
		Oauth2Client: &req.Oauth2Client, ClientSecret: plain,
	})
}

// Update PUT /admin/oauth2/clients/:id → data 回显；id 非法/bind 错 400，service 错 500。
// body 中 secret 留空 → 服务层保留库中旧值（Task 7 Update 语义），且响应经 json:"-" 不回显。
func (c *ClientController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req model.Oauth2Client
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.ClientID == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	req.ID = id
	if err := c.svc.Update(ctx.Request.Context(), &req); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, req)
}

// Delete DELETE /admin/oauth2/clients/:id → 信封 code 0。
func (c *ClientController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
