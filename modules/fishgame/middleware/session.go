package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

const (
	sessionExpire  = 30 * 24 * time.Hour
	sessionSlideAt = 7 * 24 * time.Hour
)

// SessionAuth X-Session 中间件：校验 token → c.Set("fishgame_player", *model.Player)。
// miss/过期 → 401 裸 JSON（不调 pkg/resp）。
func SessionAuth(players repository.PlayerRepository, sessions repository.SessionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Session")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		ctx := c.Request.Context()
		sess, err := sessions.GetByToken(ctx, token)
		if err != nil || sess == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		// 滑续：剩余 <7d → 顺延 now+30d（失败不阻断请求）。
		if time.Until(sess.ExpireAt) < sessionSlideAt {
			_ = sessions.TouchExpire(ctx, sess.ID, time.Now().Add(sessionExpire))
		}
		player, err := players.GetByID(ctx, sess.PlayerID)
		if err != nil || player == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("fishgame_player", *player)
		c.Next()
	}
}

// PlayerFrom 从 gin.Context 取已认证 player（控制器用）。
func PlayerFrom(c *gin.Context) (model.Player, bool) {
	v, ok := c.Get("fishgame_player")
	if !ok {
		return model.Player{}, false
	}
	p, ok := v.(model.Player)
	return p, ok
}
