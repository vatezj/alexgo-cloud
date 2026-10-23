package service

import (
	"context"
	"fmt"
	"strings"

	"alexGo-cloud/modules/fishgame/repository"
)

// AuthService 登录（wechat code2session → player upsert → session 签发）。
type AuthService struct {
	wechat   *WechatClient
	players  repository.PlayerRepository
	sessions *SessionService
}

func NewAuthService(wechat *WechatClient, players repository.PlayerRepository, sessionRepo repository.SessionRepository) *AuthService {
	return &AuthService{
		wechat:   wechat,
		players:  players,
		sessions: NewSessionService(sessionRepo, players),
	}
}

// Login 换 openid、幂等落 player、签发 session token。失败返回 error（调用方映射 401 login_failed）。
func (s *AuthService) Login(ctx context.Context, code string) (string, error) {
	openid, err := s.wechat.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	// nick 占位：微信用户+openid 尾4；清 ":" 防串。
	tail := openid
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	nick := "微信用户" + strings.ReplaceAll(tail, ":", "")
	p, err := s.players.UpsertByOpenID(ctx, openid, nick, "")
	if err != nil {
		return "", fmt.Errorf("upsert player: %w", err)
	}
	return s.sessions.Issue(ctx, p.ID)
}
