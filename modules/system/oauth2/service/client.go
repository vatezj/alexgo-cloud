package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/modules/system/oauth2/repository"
)

// ErrInvalidClient 未知/停用/凭据不符（Task 9 映射 400 invalid_client、Task 10 映射 401 invalid_client）。
var ErrInvalidClient = errors.New("invalid client")

type ClientService struct {
	repo *repository.ClientRepository
}

func NewClientService(repo *repository.ClientRepository) *ClientService {
	return &ClientService{repo: repo}
}

// Get 取启用中的 client（authorize 端点入口校验，Task 9）。
func (s *ClientService) Get(ctx context.Context, clientID string) (*model.Oauth2Client, error) {
	c, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil || c == nil || c.Activity != 1 {
		return nil, ErrInvalidClient
	}
	return c, nil
}

// Authenticate client 凭据校验（Task 10 token 端点第一步）：
// 库中 secret 空 = 公开客户端跳过比对（D10，授权码流靠 PKCE）；
// 机密客户端 bcrypt 比对；未知/停用/不符一律 ErrInvalidClient（不区分原因，防探测）。
func (s *ClientService) Authenticate(ctx context.Context, clientID, clientSecret string) (*model.Oauth2Client, error) {
	c, err := s.repo.GetByClientID(ctx, clientID)
	if err != nil || c == nil || c.Activity != 1 {
		return nil, ErrInvalidClient
	}
	if c.ClientSecret == "" {
		return c, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(c.ClientSecret), []byte(clientSecret)) != nil {
		return nil, ErrInvalidClient
	}
	return c, nil
}

func (s *ClientService) List(ctx context.Context, page, size int) ([]*model.Oauth2Client, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	return s.repo.List(ctx, page, size)
}

// Create 校验并落库：client_id 非空；secret 非空 → bcrypt 存哈希（明文只在请求里出现一次）；
// Activity 零值补 1（新建即启用，停用走 Update）；Scopes 空补 "all"。
func (s *ClientService) Create(ctx context.Context, c *model.Oauth2Client) error {
	if c.ClientID == "" {
		return fmt.Errorf("client_id required")
	}
	if c.ClientSecret != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(c.ClientSecret), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		c.ClientSecret = string(h)
	}
	if c.Activity == 0 {
		c.Activity = 1
	}
	if c.Scopes == "" {
		c.Scopes = "all"
	}
	return s.repo.Create(ctx, c)
}

// Update secret 空 → 保留库中旧值（编辑表单留空即不改机密）；非空 → 重哈希。
func (s *ClientService) Update(ctx context.Context, c *model.Oauth2Client) error {
	if c.ID == 0 {
		return fmt.Errorf("invalid id")
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id required")
	}
	old, err := s.repo.GetByID(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if c.ClientSecret == "" {
		c.ClientSecret = old.ClientSecret
	} else {
		h, herr := bcrypt.GenerateFromPassword([]byte(c.ClientSecret), bcrypt.DefaultCost)
		if herr != nil {
			return herr
		}
		c.ClientSecret = string(h)
	}
	if c.Scopes == "" {
		c.Scopes = "all"
	}
	return s.repo.Update(ctx, c)
}

func (s *ClientService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("invalid id")
	}
	return s.repo.Delete(ctx, id)
}
