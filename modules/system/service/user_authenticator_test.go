package service

import (
	"context"
	"errors"
	"testing"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/modules/system/model"
)

func TestNewUserAuthenticator_BadPassword(t *testing.T) {
	svc := NewUserAuthenticator(&fakeUserRepo{user: enabledUser()}, fakePerm{})
	_, err := svc.Authenticate(context.Background(), "alice", "wrong")
	if !errors.Is(err, oauth2svc.ErrInvalidCredentials) {
		t.Fatalf("错密码应 ErrInvalidCredentials, got %v", err)
	}
}

func TestNewUserAuthenticator_OK(t *testing.T) {
	repo := &fakeUserRepo{user: enabledUser()}
	svc := NewUserAuthenticator(repo, fakePerm{})
	ident, err := svc.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if ident.UserID != 3 || ident.Username != "alice" || ident.AccountType != oauth2svc.AccountTypeSystem {
		t.Errorf("identity = %+v", ident)
	}
}

func TestNewUserAuthenticator_Disabled(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: 6, Username: "x", Status: 0, PasswordHash: hashOf("pw")}}
	svc := NewUserAuthenticator(repo, fakePerm{})
	_, err := svc.Authenticate(context.Background(), "x", "pw")
	if !errors.Is(err, oauth2svc.ErrUserDisabled) {
		t.Fatalf("停用应 ErrUserDisabled, got %v", err)
	}
}
