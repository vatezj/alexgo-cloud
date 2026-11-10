package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alexGo-cloud/modules/fishgame/repository"
)

// AuthService 登录（wechat code2session → player upsert → session 签发）。
type AuthService struct {
	wechat   *WechatClient
	players  repository.PlayerRepository
	sessions *SessionService
	// testAccounts / testSecret：编辑器测试登录双闸（名单+密钥），见 TestLogin。
	testAccounts []string
	testSecret   string
}

func NewAuthService(wechat *WechatClient, players repository.PlayerRepository, sessionRepo repository.SessionRepository, testAccounts []string, testSecret string) *AuthService {
	return &AuthService{
		wechat:       wechat,
		players:      players,
		sessions:     NewSessionService(sessionRepo, players),
		testAccounts: testAccounts,
		testSecret:   testSecret,
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

// TestAccounts 测试账号名单（编辑器面板拉取；副本防外部改内部切片）。
func (s *AuthService) TestAccounts() []string {
	out := make([]string, len(s.testAccounts))
	copy(out, s.testAccounts)
	return out
}

// TestLogin 编辑器测试账号换会话：密钥 + 名单双闸，通过则合成 openid
// "test_<account>" 幂等建号（首登自动创建）并签发 session。任一闸不过返回
// 统一 error（调用方映射 401 login_failed，不区分原因防探测）。
func (s *AuthService) TestLogin(ctx context.Context, account, secret string) (string, error) {
	if account == "" || secret != s.testSecret {
		return "", errors.New("test login denied")
	}
	inList := false
	for _, a := range s.testAccounts {
		if a == account {
			inList = true
			break
		}
	}
	if !inList {
		return "", errors.New("test account not in list")
	}
	p, err := s.players.UpsertByOpenID(ctx, "test_"+account, "测试_"+account, "")
	if err != nil {
		return "", fmt.Errorf("upsert test player: %w", err)
	}
	return s.sessions.Issue(ctx, p.ID)
}
