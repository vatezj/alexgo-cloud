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

func (ctrl *AdminController) ListUsers(c *gin.Context) {
	users, err := ctrl.userSvc.ListUsers(c.Request.Context())
	if err != nil {
		resp.Fail(c, err)
		return
	}
	resp.OK(c, users)
}
