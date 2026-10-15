package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/oauth2/model"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// ClientController OAuth2 client 管理（admin 信封 {data}/{data,total}/{status}/{error}，
// 与 system admin 控制器同款；非 RFC 6749 信封——协议信封只属于 /api/oauth2/*）。
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

// List GET /admin/oauth2/clients?page&size → {data,total}。
// page/size 归正（1/20）在 ClientService.List；本层只补 size 上限 200（裁决 4）。
func (c *ClientController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	if size > 200 {
		size = 200
	}
	items, total, err := c.svc.List(ctx.Request.Context(), page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// Create POST /admin/oauth2/clients → {data}；client_id 空先行 400（裁决 3）。
func (c *ClientController) Create(ctx *gin.Context) {
	var req createClientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "client_id required"})
		return
	}
	req.ID = 0                            // 新建忽略请求 id
	plain := req.ClientSecret             // 明文只在此刻存在（裁决 2）
	req.Oauth2Client.ClientSecret = plain // json:"-" 不参与绑定，手动写回再哈希
	if err := c.svc.Create(ctx.Request.Context(), &req.Oauth2Client); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": createClientResponse{
		Oauth2Client: &req.Oauth2Client, ClientSecret: plain,
	}})
}

// Update PUT /admin/oauth2/clients/:id → {data}；id 非法/bind 错 400，service 错 500。
// body 中 secret 留空 → 服务层保留库中旧值（Task 7 Update 语义），且响应经 json:"-" 不回显。
func (c *ClientController) Update(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req model.Oauth2Client
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "client_id required"})
		return
	}
	req.ID = id
	if err := c.svc.Update(ctx.Request.Context(), &req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": req})
}

// Delete DELETE /admin/oauth2/clients/:id → {"status":"ok"}。
func (c *ClientController) Delete(ctx *gin.Context) {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
