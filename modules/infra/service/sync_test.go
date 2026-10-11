package service

import (
	"context"
	"testing"

	"alexGo-cloud/modules/infra/model"
)

// 钉子①（Review Focus #2）：同步绝不覆盖配置字段——list/query 开关保持原值。
func TestSync_ConfigSwitchesSurvive(t *testing.T) {
	svc, _, reader := newSvc(t)

	// 导入后把 name 列的配置改掉（模拟用户编辑）
	tbl, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatal(err)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	nameCol := cols[1]
	nameCol.ListEnable = false
	nameCol.QueryEnable = true
	nameCol.QueryOperation = "like"
	nameCol.HTMLType = "Select"
	if err := svc.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{nameCol}); err != nil {
		t.Fatal(err)
	}

	// 库表没变：只改注释逼出 Refresh
	meta := reader.tables["order_items"]
	meta.Columns[1].Comment = "改了注释"
	meta.Comment = "订单明细v2"

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Updated != 1 || res.Added != 0 || res.Deprecated != 0 || res.Unchanged != 2 {
		t.Errorf("res = %+v", res)
	}

	_, cols2, _ := svc.GetTable(context.Background(), tbl.ID)
	var after model.CodegenColumn
	for _, c := range cols2 {
		if c.Name == "name" {
			after = *c
		}
	}
	if after.ListEnable || !after.QueryEnable || after.QueryOperation != "like" || after.HTMLType != "Select" {
		t.Errorf("配置字段被同步覆盖: %+v", after)
	}
	if after.Comment != "改了注释" {
		t.Errorf("快照未刷新: comment=%q", after.Comment)
	}
}

// 钉子②：消失列只标 deprecated，行仍在（不物理删）。
func TestSync_MissingColumn_MarkedDeprecatedNotDeleted(t *testing.T) {
	svc, _, reader := newSvc(t)
	tbl, err := svc.ImportTable(context.Background(), "order_items")
	if err != nil {
		t.Fatal(err)
	}

	// 库表删掉 qty 列
	m := reader.tables["order_items"]
	m.Columns = m.Columns[:2] // id, name
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deprecated != 1 || res.Updated != 0 || res.Added != 0 {
		t.Errorf("res = %+v", res)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	if len(cols) != 3 {
		t.Fatalf("行被物理删: cols=%d, want 3", len(cols))
	}
	var found bool
	for _, c := range cols {
		if c.Name == "qty" {
			found = true
			if !c.Deprecated {
				t.Error("qty 未标 deprecated")
			}
		}
	}
	if !found {
		t.Fatal("qty 行消失")
	}
}

// 钉子③：deprecated 列重新出现 → 恢复 + 计入 Updated，配置字段保留。
func TestSync_Reappear_RestoresDeprecated(t *testing.T) {
	svc, _, reader := newSvc(t)
	tbl, _ := svc.ImportTable(context.Background(), "order_items")
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	qty := cols[2]
	qty.QueryEnable = true
	qty.QueryOperation = "gt"
	if err := svc.SaveColumns(context.Background(), tbl.ID, []*model.CodegenColumn{qty}); err != nil {
		t.Fatal(err)
	}

	// 删列 → 同步 → 恢复列 → 同步
	m := reader.tables["order_items"]
	m.Columns = m.Columns[:2]
	if _, err := svc.Sync(context.Background(), tbl.ID); err != nil {
		t.Fatal(err)
	}
	m.Columns = append(m.Columns, col("qty", "int", "", ""))
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Deprecated != 0 {
		t.Errorf("res = %+v", res)
	}
	_, cols2, _ := svc.GetTable(context.Background(), tbl.ID)
	for _, c := range cols2 {
		if c.Name == "qty" {
			if c.Deprecated {
				t.Error("恢复后仍 deprecated")
			}
			if !c.QueryEnable || c.QueryOperation != "gt" {
				t.Errorf("恢复时配置丢失: %+v", c)
			}
		}
	}
}

// 新列：开关全关 + sort_order 接在末尾（§9.4）。
func TestSync_NewColumn_AllSwitchesOff(t *testing.T) {
	svc, _, reader := newSvc(t)
	tbl, _ := svc.ImportTable(context.Background(), "order_items")
	m := reader.tables["order_items"]
	m.Columns = append(m.Columns, col("remark", "varchar(255)", "", ""))
	reader.tables["order_items"] = m

	res, err := svc.Sync(context.Background(), tbl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("res = %+v", res)
	}
	_, cols, _ := svc.GetTable(context.Background(), tbl.ID)
	var remark *model.CodegenColumn
	for _, c := range cols {
		if c.Name == "remark" {
			remark = c
		}
	}
	if remark == nil {
		t.Fatal("remark 未落库")
	}
	if remark.ListEnable || remark.FormEnable || remark.QueryEnable {
		t.Errorf("新列开关应全关: %+v", remark)
	}
	if remark.SortOrder != 3 {
		t.Errorf("sort_order = %d, want 3（既有最大 2 + 1）", remark.SortOrder)
	}
}
