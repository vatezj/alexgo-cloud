package admin

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/resp"
)

type AdminController struct {
	userSvc service.UserService
}

func NewAdminController(userSvc service.UserService) *AdminController {
	return &AdminController{userSvc: userSvc}
}

// ListUsers godoc
//
//	@Summary		用户列表（全量）
//	@Description	权限 system:user；当前租户全量用户，无分页。统一 pkg/resp 信封。
//	@Tags			system-admin
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Router			/admin/system/users [get]
func (ctrl *AdminController) ListUsers(c *gin.Context) {
	users, err := ctrl.userSvc.ListUsers(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, users)
}
