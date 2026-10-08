package gormplugin

import (
	"context"
	"sort"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/pkg/tenant"
)

type row struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (row) TableName() string { return "rows" }

type noTenantRow struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func (noTenantRow) TableName() string { return "no_tenant_rows" }

func setup(t *testing.T, opts Options) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&row{}, &noTenantRow{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(db, opts); err != nil {
		t.Fatal(err)
	}
	return db
}

// INSERT 自动填 tenant_id；SELECT 只见本租户行（跨租户行互不可见）。
func TestIsolation_CrossTenant(t *testing.T) {
	db := setup(t, Options{})

	ctx1 := tenant.WithTenantID(context.Background(), 1)
	ctx2 := tenant.WithTenantID(context.Background(), 2)

	if err := db.WithContext(ctx1).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(ctx2).Create(&row{Name: "b"}).Error; err != nil {
		t.Fatal(err)
	}

	var got []row
	if err := db.WithContext(ctx1).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" || got[0].TenantID != 1 {
		t.Fatalf("tenant1 sees %+v, want only own row with injected tenant_id", got)
	}

	var got2 []row
	if err := db.WithContext(ctx2).Find(&got2).Error; err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].Name != "b" || got2[0].TenantID != 2 {
		t.Fatalf("tenant2 sees %+v", got2)
	}
}

// tid=0（未解析/平台）：不过滤不填充——保持历史行为：
// 平台侧显式写入他租户行（seed/回填）与 0 行都可见。
func TestTenantZero_NoInjection(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(context.Background()).Create(&row{Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(context.Background()).Create(&row{Name: "seed", TenantID: 5}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(context.Background()).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("tid=0 must not filter: %+v", got)
	}
	for _, g := range got {
		if g.Name == "x" && g.TenantID != 0 {
			t.Fatalf("tid=0 must not inject: %+v", g)
		}
		if g.Name == "seed" && g.TenantID != 5 {
			t.Fatalf("explicit tenant_id must be preserved: %+v", g)
		}
	}
}

// 无 TenantID 字段的模型完全不受影响。
func TestNoTenantField_Skip(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 9)).Create(&noTenantRow{Name: "n"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []noTenantRow
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 9)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

// 白名单表跳过过滤（也不回填）。
func TestExemptTable(t *testing.T) {
	db := setup(t, Options{ExemptTables: []string{"rows"}})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "e", TenantID: 77}).Error; err != nil {
		t.Fatal(err)
	}
	// 零值行也不得被回填（白名单完全不参与注入）。
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "zero"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 2)).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("exempt table must not be filtered: %+v", got)
	}
	for _, g := range got {
		switch g.Name {
		case "e":
			if g.TenantID != 77 {
				t.Fatalf("explicit tenant_id must be preserved: %+v", g)
			}
		case "zero":
			if g.TenantID != 0 {
				t.Fatalf("exempt table must not be filled: %+v", g)
			}
		}
	}
}

// IgnoreTenant 显式放行（平台侧全量查询）：ctx 同时带 tenant_id 与 ignore 标记，
// 必须绕过过滤看到所有租户的行。
func TestIgnoreTenant(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 2)).Create(&row{Name: "b"}).Error; err != nil {
		t.Fatal(err)
	}

	// 不带 ignore：只见到本租户 1 行（对照组）。
	var scoped []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Find(&scoped).Error; err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Name != "a" {
		t.Fatalf("scoped query sees %+v, want only tenant1 row", scoped)
	}

	// 带 ignore：全量 2 行。
	var got []row
	if err := db.WithContext(IgnoreTenant(tenant.WithTenantID(context.Background(), 1))).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("IgnoreTenant sees %+v, want all", got)
	}
}

// Update/Delete 同样追加租户条件（本任务统一处理，T10 才区分 scope）。
func TestUpdateDelete_CrossTenantFiltered(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	var seed []row
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Find(&seed).Error; err != nil || len(seed) != 1 {
		t.Fatalf("seed: %+v err=%v", seed, err)
	}
	id := seed[0].ID

	// 跨租户 UPDATE 不得命中。
	res := db.WithContext(tenant.WithTenantID(context.Background(), 2)).
		Model(&row{}).Where("id = ?", id).Update("name", "hacked")
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("cross-tenant update affected %d rows, want 0", res.RowsAffected)
	}

	// 跨租户 DELETE 不得命中。
	res = db.WithContext(tenant.WithTenantID(context.Background(), 2)).Where("id = ?", id).Delete(&row{})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 0 {
		t.Fatalf("cross-tenant delete affected %d rows, want 0", res.RowsAffected)
	}

	// 本租户 UPDATE/DELETE 正常。
	res = db.WithContext(tenant.WithTenantID(context.Background(), 1)).
		Model(&row{}).Where("id = ?", id).Update("name", "a2")
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("owner update affected=%d err=%v", res.RowsAffected, res.Error)
	}
	res = db.WithContext(tenant.WithTenantID(context.Background(), 1)).Where("id = ?", id).Delete(&row{})
	if res.Error != nil || res.RowsAffected != 1 {
		t.Fatalf("owner delete affected=%d err=%v", res.RowsAffected, res.Error)
	}
}

// ---- Task 10：data_scope 五档查询注入 ----

type empRow struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string
	DeptID   uint64 `gorm:"column:dept_id"`
	TenantID uint64 `gorm:"column:tenant_id"`
}

func (empRow) TableName() string { return "emp_rows" }

func setupEmp(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&empRow{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(db, Options{}); err != nil {
		t.Fatal(err)
	}
	seed := []empRow{
		{Name: "u1", DeptID: 10, TenantID: 1},
		{Name: "u2", DeptID: 20, TenantID: 1},
		{Name: "u3", DeptID: 21, TenantID: 1},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func names(t *testing.T, db *gorm.DB, ctx context.Context) []string {
	t.Helper()
	var rows []empRow
	if err := db.WithContext(ctx).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

func baseCtx() context.Context {
	return tenant.WithTenantID(context.Background(), 1)
}

func TestDataScope_AllModes(t *testing.T) {
	db := setupEmp(t)

	// Mode 1 全部
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 1, UserID: 1, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 3 {
		t.Errorf("mode1 = %v, want 3 rows", got)
	}

	// Mode 2 自定义 [20,21]；UserID 故意给不存在的 999——判别依据必须是部门集合，
	// 若误按 id=UserID 过滤则 0 行、若不过滤则 3 行，两种错法都会被断言抓住。
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 2, UserID: 999, DeptIDs: []uint64{20, 21}})
	if got := names(t, db, ctx); len(got) != 2 || got[0] != "u2" || got[1] != "u3" {
		t.Errorf("mode2 = %v", got)
	}

	// Mode 3 本部门（dept 10）；同理 UserID=999，只有 dept 条件能命中 u1。
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 3, UserID: 999, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode3 = %v", got)
	}

	// Mode 4 本部门及以下（Loader 算出 10,11 —— 种子没有 dept 11 的人，只有 10）
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 4, UserID: 999, DeptID: 10, DeptIDs: []uint64{10, 11}})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode4 = %v", got)
	}

	// Mode 5 仅本人（user id=1 → 只有 u1 是 seed 第一行 id=1）
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 5, UserID: 1, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode5 = %v", got)
	}
	// Mode 5 判别①：DeptID 指向别处（20）也必须按 id=UserID 命中 u1（dept 条件会错给 u2）。
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 5, UserID: 1, DeptID: 20})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode5 wrong-dept = %v", got)
	}
	// Mode 5 判别②：UserID 不存在 → 0 行（dept/no-filter 两种错法分别给 1/3 行）。
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 5, UserID: 999, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 0 {
		t.Errorf("mode5 unknown user = %v, want []", got)
	}

	// 无 scope 注入（老行为）：不过滤
	if got := names(t, db, baseCtx()); len(got) != 3 {
		t.Errorf("no scope = %v", got)
	}
}

// 没有 DeptID 字段的模型不受 data scope 影响。
func TestDataScope_NoDeptField_Skip(t *testing.T) {
	db := setup(t, Options{})
	ctx := tenant.WithDataScope(tenant.WithTenantID(context.Background(), 1),
		tenant.DataScope{Mode: 5, UserID: 999})
	if err := db.WithContext(ctx).Create(&row{Name: "n"}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	if err := db.WithContext(ctx).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}

// scope 只作用于 Query：Update/Delete 仅走租户过滤
// （scope 是列表可见性下限，不能把管理端按主键/条件的更新误杀）。
func TestDataScope_UpdateDelete_NotScoped(t *testing.T) {
	db := setupEmp(t)
	// Mode 5（仅本人 user 1）注入后，Update/Delete 仍应命中其余行。
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 5, UserID: 1, DeptID: 10})

	res := db.WithContext(ctx).Model(&empRow{}).Where("id <> ?", 1).Update("name", "x")
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 2 {
		t.Errorf("update affected = %d, want 2 (scope must not filter update)", res.RowsAffected)
	}

	res = db.WithContext(ctx).Where("id <> ?", 1).Delete(&empRow{})
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	if res.RowsAffected != 2 {
		t.Errorf("delete affected = %d, want 2 (scope must not filter delete)", res.RowsAffected)
	}
}

// Mode 4 与 Mode 3 的区别：Mode 4 吃整个部门集合（含子部门行），
// Mode 3 只吃本部门——判别"及以下"是否真的用了 IN 集合而不是等值。
func TestDataScope_Mode4_IncludesChildDept(t *testing.T) {
	db := setupEmp(t)
	if err := db.Create(&empRow{Name: "u11", DeptID: 11, TenantID: 1}).Error; err != nil {
		t.Fatal(err)
	}

	// Mode 3 只见本部门（10）：子部门 11 的行不得进。
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 3, UserID: 999, DeptID: 10})
	if got := names(t, db, ctx); len(got) != 1 || got[0] != "u1" {
		t.Errorf("mode3 = %v, want [u1]", got)
	}

	// Mode 4 见 10+11：子部门行必须进来。
	ctx = tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 4, UserID: 999, DeptID: 10, DeptIDs: []uint64{10, 11}})
	if got := names(t, db, ctx); len(got) != 2 || got[0] != "u1" || got[1] != "u11" {
		t.Errorf("mode4 = %v, want [u1 u11]", got)
	}
}

// Mode 2 自定义集合为空 → 显式"看不到"（0 行），而不是放开全部。
func TestDataScope_Mode2Empty_EmptyResult(t *testing.T) {
	db := setupEmp(t)
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 2, UserID: 1})
	if got := names(t, db, ctx); len(got) != 0 {
		t.Errorf("mode2 empty set = %v, want []", got)
	}
}

// Mode 4 集合为空（dept_id=0 未挂部门 / 部门树为空）→ 与 mode2 同样 fail-closed：
// 显式 0 行，绝不允许退化成"租户内全量"（那会在最宽档位上放开范围）。
func TestDataScope_Mode4Empty_FailsClosed(t *testing.T) {
	db := setupEmp(t)
	ctx := tenant.WithDataScope(baseCtx(), tenant.DataScope{Mode: 4, UserID: 1, DeptID: 0, DeptIDs: nil})
	if got := names(t, db, ctx); len(got) != 0 {
		t.Errorf("mode4 empty set = %v, want [] (fail-closed)", got)
	}
}

// INSERT 回填仅零值：非白名单表上显式赋值的 tenant_id 不得被覆盖。
func TestInsert_ExplicitTenantIDPreserved(t *testing.T) {
	db := setup(t, Options{})
	if err := db.WithContext(tenant.WithTenantID(context.Background(), 1)).Create(&row{Name: "explicit", TenantID: 42}).Error; err != nil {
		t.Fatal(err)
	}
	var got []row
	// 过滤 tenant_id=1 查不到 42；必须用全量通道读回。
	if err := db.WithContext(IgnoreTenant(context.Background())).Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TenantID != 42 {
		t.Fatalf("explicit tenant_id must be preserved: %+v", got)
	}
}
