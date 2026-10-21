package social

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTableNames(t *testing.T) {
	if got := (&SocialApp{}).TableName(); got != "system_social_app" {
		t.Errorf("SocialApp.TableName = %q", got)
	}
	if got := (&MemberIdentity{}).TableName(); got != "system_member_identity" {
		t.Errorf("MemberIdentity.TableName = %q", got)
	}
}

// 迁移 DDL 钉死唯一键/索引——漂移即红（Review Focus #1/#6 的库侧前提）。
func TestSocialTablesMigrationDDL(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "migrations", "20261021000001_social_tables.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(b)
	for _, frag := range []string{
		"CREATE TABLE IF NOT EXISTS `system_social_app`",
		"CREATE TABLE IF NOT EXISTS `system_member_identity`",
		"UNIQUE KEY `uk_social_app_tenant_platform` (`tenant_id`, `platform`)",
		"UNIQUE KEY `uk_identity_tenant_platform_openid` (`tenant_id`, `platform`, `openid`)",
		"KEY `idx_identity_tenant_unionid` (`tenant_id`, `unionid`)",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("up.sql 缺片段: %s", frag)
		}
	}
}
