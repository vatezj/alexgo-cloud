package template_test

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/codegen/builder"
	"alexGo-cloud/pkg/codegen/metadata"
	"alexGo-cloud/pkg/codegen/template"
)

// renderSingle 用 fixture 元数据渲染单表后端模板族，返回指定模板的产物源码。
func renderSingle(t *testing.T, columns []metadata.ColumnMeta, wantPathSuffix string) string {
	t.Helper()
	meta := &metadata.TableMeta{Schema: "alexgo", Name: "codegen_demo_items", Comment: "demo", Columns: columns}
	tbl, err := builder.Build(meta, builder.Options{Module: "order"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files, err := template.Render(tbl, template.Options{UnitTestEnable: true})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, wantPathSuffix) {
			return string(f.Content)
		}
	}
	t.Fatalf("no generated file ends with %q", wantPathSuffix)
	return ""
}

func tenantColumns() []metadata.ColumnMeta {
	return []metadata.ColumnMeta{
		{Name: "id", DataType: "bigint", ColumnType: "bigint unsigned", Key: "PRI", Extra: "auto_increment", Ordinal: 1},
		{Name: "name", DataType: "varchar", ColumnType: "varchar(64)", Comment: "名称", Ordinal: 2},
		{Name: "tenant_id", DataType: "bigint", ColumnType: "bigint unsigned", Comment: "租户", Ordinal: 3},
		{Name: "created_at", DataType: "datetime", ColumnType: "datetime", Comment: "创建时间", Ordinal: 4},
	}
}

// 渲染断言（spec §9.2）：租户表的产物必须隐藏 tenant_id、打戳、双条件过滤更新，且不用 Save。
func TestTenantWritePath_Render(t *testing.T) {
	repo := renderSingle(t, tenantColumns(), "repository/codegen_demo_item.go")
	for _, want := range []string{
		`"alexGo-cloud/pkg/tenant"`,
		`Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx))`,
		`Select("*").Updates(entity)`,
		`if entity.TenantID == 0 {`,
	} {
		if !strings.Contains(repo, want) {
			t.Errorf("repository 产物缺少 %q", want)
		}
	}
	if strings.Contains(repo, `Save(entity)`) {
		t.Error("repository 产物含 Save(entity)：Save 丢弃链式 WHERE，跨租户照改（spec §9.2）")
	}

	svc := renderSingle(t, tenantColumns(), "service/codegen_demo_item.go")
	if got := strings.Count(svc, `entity.TenantID = tenant.TenantIDFromContext(ctx)`); got < 2 {
		t.Errorf("service 产物打戳语句 %d 处, want >= 2（Create + Update）", got)
	}

	modelSrc := renderSingle(t, tenantColumns(), "model/codegen_demo_item.go")
	for _, line := range strings.Split(modelSrc, "\n") {
		if strings.Contains(line, "TenantID") {
			if !strings.Contains(line, `json:"-"`) {
				t.Errorf("TenantID 行未隐藏 json: %s", line)
			}
			return
		}
	}
	t.Error("model 产物无 TenantID 字段")
}

// gorm 语义镜像：证明模板禁用 Save 的理由——
// ① 双条件 + Select("*").Updates 把行锁在租户内（跨租户 0 行）；
// ② 链式 WHERE 后接 Save，gorm 丢弃 WHERE 照改不误（这是要防的坑本身）。
func TestTenantWritePath_GormSemantics(t *testing.T) {
	type mirrorRow struct {
		ID       uint64 `gorm:"primaryKey"`
		TenantID uint64
		Data     string
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&mirrorRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&mirrorRow{ID: 1, TenantID: 1, Data: "original"}).Error; err != nil {
		t.Fatal(err)
	}

	// ① 模板现行写法：租户不匹配 → 0 行受影响，数据不动。
	err = db.Model(&mirrorRow{}).
		Where("id = ? AND tenant_id = ?", 1, 2).
		Select("*").Updates(&mirrorRow{ID: 1, TenantID: 2, Data: "hacked"}).Error
	if err != nil {
		t.Fatalf("Updates: %v", err)
	}
	var row mirrorRow
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "original" {
		t.Errorf("跨租户 Updates 改动了数据: %+v, want 保持 original", row)
	}

	// ② 反例（characterization of gorm footgun）：链式 WHERE 被 Save 丢弃。
	_ = db.Where("id = ? AND tenant_id = ?", 1, 2).Save(&mirrorRow{ID: 1, TenantID: 1, Data: "hacked-save"})
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "hacked-save" {
		t.Errorf("Save 行为变化: data=%q；若此处失败说明 gorm 语义已变，重新审视模板 Update 写法", row.Data)
	}
}
