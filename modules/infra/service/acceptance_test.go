package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/infra/model"
)

// TestM2_Acceptance_ImportEditSync 是 spec §8 M2 验收链：
// 导入 → 改配置（query 开关+操作符）→ 库表加列删列改注释 → 同步 → diff 计数。
func TestM2_Acceptance_ImportEditSync(t *testing.T) {
	svc, _, reader := newSvc(t)
	ctx := context.Background()

	// 1) 导入
	tbl, err := svc.ImportTable(ctx, "order_items")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	_, cols, _ := svc.GetTable(ctx, tbl.ID)
	if len(cols) != 3 {
		t.Fatalf("import cols=%d, want 3", len(cols))
	}

	// 2) 改配置：name 列 query 开、between
	var nameID uint64
	for _, c := range cols {
		if c.Name == "name" {
			nameID = c.ID
		}
	}
	if err := svc.SaveColumns(ctx, tbl.ID, []*model.CodegenColumn{{
		ID: nameID, TableID: tbl.ID, Name: "name",
		QueryEnable: true, QueryOperation: "between", HTMLType: "Select",
	}}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// 3) 库表变更：+remark 列、删 qty 列、改表注释
	m := reader.tables["order_items"]
	m.Columns = append(m.Columns[:2], col("remark", "varchar(255)", "", "")) // [id,name,remark]
	m.Comment = "订单明细v2"
	reader.tables["order_items"] = m

	// 4) 同步 → diff（§9.4）：Added=1(remark), Deprecated=1(qty),
	//    name 只改了配置→快照无变化→unchanged（配置变更不计 updated）, Unchanged=2(id,name)
	res, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("added=%d, want 1", res.Added)
	}
	if res.Deprecated != 1 {
		t.Errorf("deprecated=%d, want 1", res.Deprecated)
	}
	if res.Updated != 0 {
		t.Errorf("updated=%d, want 0（仅配置变化，快照未变）", res.Updated)
	}
	if res.Unchanged != 2 {
		t.Errorf("unchanged=%d, want 2 (id,name)", res.Unchanged)
	}

	// 5) 配置字段跨同步存活 + qty 行仍在（deprecated）+ 表注释刷新
	t2, cols2, err := svc.GetTable(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if t2.TableComment != "订单明细v2" {
		t.Errorf("table_comment=%q", t2.TableComment)
	}
	if len(cols2) != 4 {
		t.Fatalf("cols=%d, want 4 (id,name,qty,remark)", len(cols2))
	}
	for _, c := range cols2 {
		switch c.Name {
		case "name":
			if !c.QueryEnable || c.QueryOperation != "between" || c.HTMLType != "Select" {
				t.Errorf("配置丢失: %+v", c)
			}
		case "qty":
			if !c.Deprecated {
				t.Error("qty 应 deprecated")
			}
		case "remark":
			if c.ListEnable || c.FormEnable || c.QueryEnable {
				t.Errorf("新列开关应全关: %+v", c)
			}
		}
	}

	// 6) 再同步一次：全 unchanged、计数归零（幂等）
	res2, err := svc.Sync(ctx, tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Added != 0 || res2.Updated != 0 || res2.Deprecated != 0 {
		t.Errorf("二次同步非幂等: %+v", res2)
	}
}
