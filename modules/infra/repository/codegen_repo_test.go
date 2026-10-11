package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/infra/model"
)

func setupRepo(t *testing.T) (*gorm.DB, CodegenRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CodegenTable{}, &model.CodegenColumn{}); err != nil {
		t.Fatal(err)
	}
	return db, NewCodegenRepository(db)
}

func seedTable(t *testing.T, repo CodegenRepository) *model.CodegenTable {
	t.Helper()
	tbl := &model.CodegenTable{Name: "order_items", Module: "order", ClassName: "OrderItem", TemplateType: 1, FrontType: 1}
	cols := []*model.CodegenColumn{
		{Name: "id", Type: "bigint", GoType: "uint64", JSONName: "id", IsPK: true, AutoIncrement: true, SortOrder: 0},
		{Name: "name", Type: "varchar(64)", GoType: "string", JSONName: "name", SortOrder: 1},
	}
	if err := repo.CreateTable(context.Background(), tbl, cols); err != nil {
		t.Fatal(err)
	}
	return tbl
}

// CreateTable 落表+字段；同名表唯一键冲突报错。
func TestRepo_CreateTableAndCascade(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	var n int64
	db.Model(&model.CodegenColumn{}).Where("table_id = ?", tbl.ID).Count(&n)
	if n != 2 {
		t.Errorf("columns = %d, want 2", n)
	}

	dup := &model.CodegenTable{Name: "order_items", Module: "order"}
	if err := repo.CreateTable(context.Background(), dup, nil); err == nil {
		t.Error("重复 table_name 应报错（uk_codegen_table_name）")
	}

	// 分页 + total
	got, total, err := repo.ListTables(context.Background(), 1, 10)
	if err != nil || total != 1 || len(got) != 1 || got[0].ID != tbl.ID {
		t.Errorf("ListTables = %d rows total=%d err=%v", len(got), total, err)
	}
}

// DeleteTable 硬删除并级联删字段（spec §9.3：无 deleted 列）。
func TestRepo_DeleteTable_CascadesColumns(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	if err := repo.DeleteTable(context.Background(), tbl.ID); err != nil {
		t.Fatal(err)
	}
	var tc, cc int64
	db.Model(&model.CodegenTable{}).Count(&tc)
	db.Model(&model.CodegenColumn{}).Count(&cc)
	if tc != 0 || cc != 0 {
		t.Errorf("after delete: tables=%d columns=%d, want 0/0", tc, cc)
	}
	if _, err := repo.GetTable(context.Background(), tbl.ID); err == nil {
		t.Error("已删行 GetTable 应报错")
	}
}

// SaveColumns 只覆盖配置字段：快照字段（type/comment/go_type/...）不被动。
func TestRepo_SaveColumns_ConfigOnly(t *testing.T) {
	db, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	cols, err := repo.ListColumns(context.Background(), tbl.ID)
	if err != nil || len(cols) != 2 {
		t.Fatalf("ListColumns = %d, err=%v", len(cols), err)
	}
	target := cols[1] // name 列
	origComment, origType := target.Comment, target.Type
	target.ListEnable = false
	target.HTMLType = "Select"
	target.QueryOperation = "between"
	target.Comment = "想覆盖快照的攻击值"
	target.Type = "hacked"
	if err := repo.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{target}); err != nil {
		t.Fatal(err)
	}

	var after model.CodegenColumn
	db.First(&after, target.ID)
	if after.ListEnable || after.HTMLType != "Select" || after.QueryOperation != "between" {
		t.Errorf("配置字段未保存: %+v", after)
	}
	if after.Comment != origComment || after.Type != origType {
		t.Errorf("快照字段被覆盖: comment %q→%q type %q→%q", origComment, after.Comment, origType, after.Type)
	}

	// 跨表 id（table_id 不匹配）→ 拒绝且 0 行改动
	other := *target
	other.ID = 999999
	other.ListEnable = false
	if err := repo.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{&other}); err == nil {
		t.Error("table_id 不匹配应报错")
	}
}

// ImportedTableNames / ListAllTables：导入去重与类名冲突检测的数据源。
func TestRepo_ImportHelpers(t *testing.T) {
	_, repo := setupRepo(t)
	tbl := seedTable(t, repo)

	names, err := repo.ImportedTableNames(context.Background())
	if err != nil || len(names) != 1 || names[0] != "order_items" {
		t.Errorf("ImportedTableNames = %v, err=%v", names, err)
	}
	all, err := repo.ListAllTables(context.Background())
	if err != nil || len(all) != 1 || all[0].ClassName != tbl.ClassName {
		t.Errorf("ListAllTables = %d rows err=%v", len(all), err)
	}
}
