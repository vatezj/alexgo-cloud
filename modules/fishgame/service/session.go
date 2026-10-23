package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/modules/fishgame/repository"
)

const (
	sessionExpire  = 30 * 24 * time.Hour // 30 天
	sessionSlideAt = 7 * 24 * time.Hour  // 剩余 <7d 滑续
)

// SessionService 会话签发/校验/滑续（spec §4.2）。
type SessionService struct {
	sessions repository.SessionRepository
	players  repository.PlayerRepository
}

func NewSessionService(sessions repository.SessionRepository, players repository.PlayerRepository) *SessionService {
	return &SessionService{sessions: sessions, players: players}
}

// newToken crypto/rand 32B base64url。
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Issue 为 player 签发新会话（tenant 恒 0，游戏端钉底）。
func (s *SessionService) Issue(ctx context.Context, playerID uint64) (string, error) {
	tok, err := newToken()
	if err != nil {
		return "", err
	}
	sess := &model.Session{
		TenantID: 0,
		Token:    tok,
		PlayerID: playerID,
		ExpireAt: time.Now().Add(sessionExpire),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return "", err
	}
	return tok, nil
}

// Verify 校验 token 并返回 player；miss/过期 → (nil, nil)。
// 剩余 <7d 时同步滑续至 now+30d（TouchExpire）。
func (s *SessionService) Verify(ctx context.Context, token string) (*model.Player, error) {
	if token == "" {
		return nil, nil
	}
	sess, err := s.sessions.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	if remain := time.Until(sess.ExpireAt); remain < sessionSlideAt {
		if err := s.sessions.TouchExpire(ctx, sess.ID, time.Now().Add(sessionExpire)); err != nil {
			return nil, err
		}
	}
	return s.players.GetByID(ctx, sess.PlayerID)
}
