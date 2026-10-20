package service

import (
	"context"
	"sort"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/modules/member/repository"
	"alexGo-cloud/pkg/tenant/gormplugin"
)

// briefRepoStub 只把 GetBriefs 代理到真实现（内存 sqlite）；嵌入 nil 接口让
// 其余方法调用即 panic——本测试只走 GetBriefs（任务简报约定的 3 行 stub）。
type briefRepoStub struct {
	repository.MemberRepository
	db *gorm.DB
}

func (s *briefRepoStub) GetBriefs(ctx context.Context, ids []uint64) ([]*model.MemberUser, error) {
	return repository.NewMemberRepository(s.db).GetBriefs(ctx, ids)
}

// GetBriefs 的 memRepo 补充实现声明在本文件而非 member_test.go（后者不在本任务
// 改动清单）：MemberRepository 接口扩展后 *memRepo 必须补齐该方法才能继续满足
// 接口、保住 NewMemberService 的既有调用点。口径与真实现一致：跳过已删、id 升序。
func (m *memRepo) GetBriefs(_ context.Context, ids []uint64) ([]*model.MemberUser, error) {
	want := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []*model.MemberUser
	for _, u := range m.byMobile {
		if want[u.ID] && !u.Deleted {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func TestGetBriefs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.MemberUser{Nickname: "甲", Avatar: "a1", Status: 1})
	db.Create(&model.MemberUser{Nickname: "乙", Avatar: "b1", Status: 1})

	svc := &memberService{repo: &briefRepoStub{db: db}}
	got, err := svc.GetBriefs(context.Background(), []uint64{1, 2, 99})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 briefs (不存在的 99 跳过), got %d", len(got))
	}
	if got[0].Nickname != "甲" || got[1].Nickname != "乙" {
		t.Fatalf("unexpected: %+v", got)
	}
}

// GetBriefs 必须跳过软删行（deleted=1）——与 memberRepo 其余查询同口径
// （model.Deleted 是普通 bool 列、非 gorm.DeletedAt，不会被 GORM 自动过滤）。
func TestGetBriefs_SkipDeleted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MemberUser{}); err != nil {
		t.Fatal(err)
	}
	if err := gormplugin.Register(db, gormplugin.Options{ExemptTables: []string{"tenants", "casbin_rule"}}); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.MemberUser{Nickname: "甲", Avatar: "a1", Status: 1})
	db.Create(&model.MemberUser{Nickname: "丙", Avatar: "c1", Status: 1, Deleted: true})

	svc := &memberService{repo: &briefRepoStub{db: db}}
	got, err := svc.GetBriefs(context.Background(), []uint64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Nickname != "甲" {
		t.Fatalf("已删行必须被跳过, got %+v", got)
	}
}
