package controller

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/modules/taokouling/repository"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/tenant"
)

// AdminController admin 五端点（casbin 中间件按 /api/admin/taokouling/* 前缀鉴权，
// handler 不取 claims；平台级数据入口恒钉 tid=0）。
type AdminController struct {
	channels *repository.ChannelRepository
	logs     *repository.ConversionLogRepository
}

// NewAdminController 装配 admin 控制器。
func NewAdminController(channels *repository.ChannelRepository, logs *repository.ConversionLogRepository) *AdminController {
	return &AdminController{channels: channels, logs: logs}
}

// channelReq POST/PUT 绑参：pid/apikey/tb_name 必填（空 → 10001）。
type channelReq struct {
	Name    string `json:"name"`
	Partner string `json:"partner"`
	Host    string `json:"host"`
	PID     string `json:"pid"`
	APIKey  string `json:"apikey"`
	TBName  string `json:"tb_name"`
	Status  int8   `json:"status"`
}

// validate 必填三元组校验；缺任一 → CodeParamInvalid（spec 未扩码，查重同归参数校验）。
func (r *channelReq) validate() error {
	if r.PID == "" || r.APIKey == "" || r.TBName == "" {
		return errs.New(errs.CodeParamInvalid, nil)
	}
	return nil
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

// pinZero 平台级数据：入口把租户钉回 0（gin ctx 经 Request 重包，fishgame pinTidZero 同款；
// repo 自钉 R1b 双保险）。
func pinZero(ctx *gin.Context) {
	ctx.Request = ctx.Request.WithContext(tenant.WithTenantID(ctx.Request.Context(), 0))
}

// ListChannels 渠道分页（GET /channels）。
//
//	@Summary	渠道分页
//	@Tags		taokouling-admin
//	@Param		page	query		integer	false	"页码，默认 1"	Example(1)
//	@Param		size	query		integer	false	"每页条数，默认 10"	Example(10)
//	@Success	200		{object}	map[string]interface{}
//	@Router		/admin/taokouling/channels [get]
func (c *AdminController) ListChannels(ctx *gin.Context) {
	pinZero(ctx)
	var req struct {
		resp.PageReq
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	list, total, err := c.channels.List(ctx.Request.Context(), req.Page, req.Size)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}

// CreateChannel 新增渠道（POST /channels）：必填校验 → 查重（partner+host，exclude=0）→ Create。
//
//	@Summary	新增渠道
//	@Tags		taokouling-admin
//	@Param		body	body		object	true	"{name,partner,host,pid,apikey,tb_name,status}"
//	@Success	200		{object}	map[string]interface{}
//	@Router		/admin/taokouling/channels [post]
func (c *AdminController) CreateChannel(ctx *gin.Context) {
	pinZero(ctx)
	var req channelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := req.validate(); err != nil {
		resp.Fail(ctx, err)
		return
	}
	dup, err := c.channels.ExistsPair(ctx.Request.Context(), req.Partner, req.Host, 0)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if dup {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	row := &model.Channel{
		Name: req.Name, Partner: req.Partner, Host: req.Host,
		PID: req.PID, APIKey: req.APIKey, TBName: req.TBName, Status: req.Status,
	}
	if err := c.channels.Create(ctx.Request.Context(), row); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, row)
}

// UpdateChannel 更新渠道（PUT /channels/:id）：GetByID miss → 10001；同必填+查重（exclude=路径 id）→ Update。
//
//	@Summary	更新渠道
//	@Tags		taokouling-admin
//	@Param		id	path		int	true	"渠道 ID"	Example(1)
//	@Param		body	body		object	true	"{name,partner,host,pid,apikey,tb_name,status}"
//	@Success	200		{object}	map[string]interface{}
//	@Router		/admin/taokouling/channels/{id} [put]
func (c *AdminController) UpdateChannel(ctx *gin.Context) {
	pinZero(ctx)
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	existing, err := c.channels.GetByID(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if existing == nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	var req channelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if err := req.validate(); err != nil {
		resp.Fail(ctx, err)
		return
	}
	dup, err := c.channels.ExistsPair(ctx.Request.Context(), req.Partner, req.Host, id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if dup {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	existing.Name = req.Name
	existing.Partner = req.Partner
	existing.Host = req.Host
	existing.PID = req.PID
	existing.APIKey = req.APIKey
	existing.TBName = req.TBName
	existing.Status = req.Status
	if err := c.channels.Update(ctx.Request.Context(), existing); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, existing)
}

// DeleteChannel 删除渠道（DELETE /channels/:id）：GetByID miss → 10001；
// 默认行（partner=="" && host==""）→ 10001（默认渠道不可删除）；Delete → OK(nil)。
//
//	@Summary	删除渠道
//	@Tags		taokouling-admin
//	@Param		id	path		int	true	"渠道 ID"	Example(2)
//	@Success	200		{object}	map[string]interface{}
//	@Router		/admin/taokouling/channels/{id} [delete]
func (c *AdminController) DeleteChannel(ctx *gin.Context) {
	pinZero(ctx)
	id, ok := paramID(ctx)
	if !ok {
		return
	}
	existing, err := c.channels.GetByID(ctx.Request.Context(), id)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	if existing == nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if existing.Partner == "" && existing.Host == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil)) // 默认渠道不可删除
		return
	}
	if err := c.channels.Delete(ctx.Request.Context(), id); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// logReq GET /logs 查询入参：start/end RFC3339 或 2006-01-02（date-only 接受）；
// status 缺省 -1（不过滤），显式传值才生效。
type logReq struct {
	resp.PageReq
	Start   string `form:"start"`
	End     string `form:"end"`
	Partner string `form:"partner"`
	ItemID  string `form:"item_id"`
	Status  *int   `form:"status"`
}

// parseTime 解析时间入参：空 → 零值（不过滤）；RFC3339 优先，退 2006-01-02；均败 → 10001。
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// ListLogs 转换记录分页（GET /logs）。
//
//	@Summary	转换记录分页
//	@Tags		taokouling-admin
//	@Param		page	query		integer	false	"页码，默认 1"	Example(1)
//	@Param		size	query		integer	false	"每页条数，默认 10"	Example(10)
//	@Param		start	query		string	false	"RFC3339 或 2006-01-02"	Example(2026-11-24)
//	@Param		end		query		string	false	"RFC3339 或 2006-01-02"	Example(2026-11-24)
//	@Param		partner	query		string	false	"渠道短码精确过滤"	Example(kp)
//	@Param		item_id	query		string	false	"商品 ID 精确过滤"	Example(123)
//	@Param		status	query		integer	false	"1成功 2降级 3无口令 4硬错误；缺省 -1 不过滤"	Example(1)
//	@Success	200		{object}	map[string]interface{}
//	@Router		/admin/taokouling/logs [get]
func (c *AdminController) ListLogs(ctx *gin.Context) {
	pinZero(ctx)
	var req logReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	req.Defaults()
	start, err := parseTime(req.Start)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	end, err := parseTime(req.End)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	status := -1 // 缺省不过滤
	if req.Status != nil {
		status = *req.Status
	}
	list, total, err := c.logs.List(ctx.Request.Context(), repository.LogFilter{
		Start: start, End: end,
		Partner: req.Partner, ItemID: req.ItemID,
		Status: status,
		Page:   req.Page, Size: req.Size,
	})
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.Page(ctx, list, total, req.PageReq)
}
