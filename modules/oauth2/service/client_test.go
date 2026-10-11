package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"alexGo-cloud/modules/oauth2/model"
	"alexGo-cloud/modules/oauth2/repository"
)

func newTestClientSvc(t *testing.T) (*ClientService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}); err != nil {
		t.Fatal(err)
	}
	return NewClientService(repository.NewClientRepository(db)), db
}

func mustCreateClient(t *testing.T, svc *ClientService, c *model.Oauth2Client) {
	t.Helper()
	if err := svc.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

// D10：公开客户端（库中 secret 空）跳过 bcrypt 比对，任意提交值放行。
func TestClientService_Authenticate_PublicSkipBcrypt(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "pub", ClientName: "公开",
		GrantTypes: "authorization_code", AccountTypes: "system",
	})
	c, err := svc.Authenticate(context.Background(), "pub", "任意secret")
	if err != nil {
		t.Fatalf("公开客户端任意 secret 应过, got %v", err)
	}
	if c.ClientID != "pub" {
		t.Errorf("client = %+v", c)
	}
}

func TestClientService_Authenticate_WrongSecret(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "conf", ClientSecret: "s3cret",
		GrantTypes: "password", AccountTypes: "member",
	})
	if _, err := svc.Authenticate(context.Background(), "conf", "nope"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("错 secret 应 ErrInvalidClient, got %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), "conf", "s3cret"); err != nil {
		t.Fatalf("正确 secret 应过, got %v", err)
	}
}

func TestClientService_Authenticate_Inactive(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{ClientID: "off", ClientSecret: "pw", GrantTypes: "password"})
	if err := db.Model(&model.Oauth2Client{}).Where("client_id = ?", "off").Update("activity", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), "off", "pw"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("停用应 ErrInvalidClient, got %v", err)
	}
	if _, err := svc.Get(context.Background(), "off"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("Get 停用应 ErrInvalidClient, got %v", err)
	}
}

func TestClientService_Get_Unknown(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	if _, err := svc.Get(context.Background(), "nope"); !errors.Is(err, ErrInvalidClient) {
		t.Fatalf("未知 client 应 ErrInvalidClient, got %v", err)
	}
}

func TestClientService_Create_HashesSecret(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{
		ClientID: "c1", ClientSecret: "plain",
		Activity: 0, Scopes: "",
	})
	var row model.Oauth2Client
	if err := db.Where("client_id = ?", "c1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ClientSecret == "plain" || bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("plain")) != nil {
		t.Errorf("secret 未 bcrypt: %q", row.ClientSecret)
	}
	if row.Activity != 1 {
		t.Errorf("Activity 应默认 1, got %d", row.Activity)
	}
	if row.Scopes != "all" {
		t.Errorf("Scopes 应默认 all, got %q", row.Scopes)
	}
	// client_id 唯一：重复创建应报错
	dup := &model.Oauth2Client{ClientID: "c1"}
	if err := svc.Create(context.Background(), dup); err == nil {
		t.Error("重复 client_id 应报错")
	}
}

func TestClientService_List_Pagination(t *testing.T) {
	svc, _ := newTestClientSvc(t)
	for i := 0; i < 3; i++ {
		mustCreateClient(t, svc, &model.Oauth2Client{
			ClientID: fmt.Sprintf("c%d", i), GrantTypes: "password", AccountTypes: "member",
		})
	}
	rows, total, err := svc.List(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || total != 3 {
		t.Errorf("rows=%d total=%d, want 2/3", len(rows), total)
	}
}

// Update secret 空 → 保留旧哈希（防编辑表单误清机密）；非空 → 重哈希。
func TestClientService_Update_SecretRules(t *testing.T) {
	svc, db := newTestClientSvc(t)
	mustCreateClient(t, svc, &model.Oauth2Client{ClientID: "u1", ClientSecret: "old", GrantTypes: "password"})

	// 空 secret：保留旧值
	c := &model.Oauth2Client{ID: 0, ClientID: "u1", ClientSecret: "", GrantTypes: "password,refresh_token"}
	var row model.Oauth2Client
	if err := db.Where("client_id = ?", "u1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	c.ID = row.ID
	if err := svc.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("old")) != nil {
		t.Errorf("空 secret 应保留旧哈希, got %q", row.ClientSecret)
	}

	// 非空 secret：重哈希
	c.ClientSecret = "new"
	if err := svc.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&row, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(row.ClientSecret), []byte("new")) != nil {
		t.Errorf("非空 secret 应重哈希, got %q", row.ClientSecret)
	}
}
