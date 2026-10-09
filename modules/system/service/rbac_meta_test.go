package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// vben generate-menus 读 meta.order ?? 999 排序——必须输出 order，
// 且 Sort=0 时省略（omitempty）以免用 0 覆盖 vben 默认 999 把 0 号排到最后。
func TestVbenRouteMeta_OrderJSON(t *testing.T) {
	b, err := json.Marshal(VbenRoute{
		Path: "/system/users",
		Meta: VbenRouteMeta{Title: "用户管理", OrderNo: 5, Order: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"order":10`) {
		t.Errorf("json = %s, want \"order\":10", s)
	}
	if !strings.Contains(s, `"orderNo":5`) {
		t.Errorf("json = %s, want orderNo 保留", s)
	}

	zero, err := json.Marshal(VbenRouteMeta{Title: "x"})
	if err != nil {
		t.Fatal(err)
	}
	// "order":（带冒号）不会误匹配 "orderNo":。
	if strings.Contains(string(zero), `"order":`) {
		t.Errorf("json = %s, want order omitted when 0", zero)
	}
}
