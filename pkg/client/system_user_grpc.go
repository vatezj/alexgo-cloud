package client

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/service"
	"google.golang.org/grpc"
)

type grpcUserClient struct {
	conn grpc.ClientConnInterface
}

func NewUserGRPCClient(conn grpc.ClientConnInterface) service.UserService {
	return &grpcUserClient{conn: conn}
}

func (c *grpcUserClient) ListUsers(ctx context.Context) ([]*model.User, error) {
	_ = ctx
	if c.conn == nil {
		return nil, fmt.Errorf("grpc conn is nil")
	}
	return nil, fmt.Errorf("grpc user client not implemented")
}

func (c *grpcUserClient) GetUserByID(ctx context.Context, id uint64) (*model.User, error) {
	_ = ctx
	_ = id
	if c.conn == nil {
		return nil, fmt.Errorf("grpc conn is nil")
	}
	return nil, fmt.Errorf("grpc user client not implemented")
}

func (c *grpcUserClient) CreateUser(ctx context.Context, username, nickname, password string) (*model.User, error) {
	_ = ctx
	_ = username
	_ = nickname
	_ = password
	if c.conn == nil {
		return nil, fmt.Errorf("grpc conn is nil")
	}
	return nil, fmt.Errorf("grpc user client not implemented")
}

func (c *grpcUserClient) UpdateUser(ctx context.Context, id uint64, nickname string, status int) error {
	_ = ctx
	_ = id
	_ = nickname
	_ = status
	if c.conn == nil {
		return fmt.Errorf("grpc conn is nil")
	}
	return fmt.Errorf("grpc user client not implemented")
}

func (c *grpcUserClient) ResetPassword(ctx context.Context, id uint64, newPassword string) error {
	_ = ctx
	_ = id
	_ = newPassword
	if c.conn == nil {
		return fmt.Errorf("grpc conn is nil")
	}
	return fmt.Errorf("grpc user client not implemented")
}

func (c *grpcUserClient) SetRoles(ctx context.Context, id uint64, roleIDs []uint64) error {
	_ = ctx
	_ = id
	_ = roleIDs
	if c.conn == nil {
		return fmt.Errorf("grpc conn is nil")
	}
	return fmt.Errorf("grpc user client not implemented")
}
