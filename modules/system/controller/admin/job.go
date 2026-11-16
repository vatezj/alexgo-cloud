package admin

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/job"
	"alexGo-cloud/pkg/resp"
)

// JobController 任务中心（pkg/job）admin 面：列表 + 手动触发。
// casbin 全局中间件按 permissionRoutes["system:job"] 前缀鉴权，handler 不取 claims。
type JobController struct {
	mgr *job.Manager
}

// NewJobController 构造任务中心控制器。
func NewJobController(mgr *job.Manager) *JobController {
	return &JobController{mgr: mgr}
}

// List godoc
//
//	@Summary		任务列表
//	@Description	权限 system:job:read；返回全部已注册任务（含停用的，带 disabledReason）与下次执行时间。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/jobs [get]
func (c *JobController) List(ctx *gin.Context) {
	resp.OK(ctx, c.mgr.List())
}

// Trigger godoc
//
//	@Summary		手动触发任务
//	@Description	权限 system:job:trigger；同步执行一次任务体（停用任务 30602、未注册 30601）。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Param			name	path	string	true	"任务名（模块.任务名）"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/jobs/{name}/trigger [post]
func (c *JobController) Trigger(ctx *gin.Context) {
	name := ctx.Param("name")
	if err := c.mgr.Trigger(ctx.Request.Context(), name); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, gin.H{"name": name})
}
