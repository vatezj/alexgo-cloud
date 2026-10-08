package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}
