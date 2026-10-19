package newmodule

import (
	"strings"
	"testing"
	"time"
)

func TestBuildReport(t *testing.T) {
	o := Options{Root: ".", Name: "demo", Entities: []string{"product", "category"},
		Now: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)}
	cs := &ChangeSet{
		Creates: []FileChange{{Path: "modules/demo/module.go"}, {Path: "modules/demo/model/product.go"}},
		Patches: []FileChange{
			{Path: "alexgo-server/cmd/main.go"},
			{Path: "alexgo-server/configs/config.yaml"},
			{Path: "modules/system/service/permission_routes.go"},
		},
	}
	rep := BuildReport(cs, o, true)
	for _, want := range []string{
		"新建模块 demo",
		"modules/demo/module.go",
		"alexgo-server/cmd/main.go",
		"docs/modules.md",
		"modules/member/cmd/main.go", // micro 提醒
		"go build ./... 通过",
		"菜单名为英文占位", // 中文化提醒
		// 回滚指引必须覆盖前端产物（终审 I2）
		"apps/web-antd/src/api/demo",
		"apps/web-antd/src/views/demo",
	} {
		if !strings.Contains(rep, want) {
			t.Errorf("报告缺少 %q\n%s", want, rep)
		}
	}
}
