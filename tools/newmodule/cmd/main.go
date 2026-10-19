// newmodule 新建模块脚手架 CLI。用法：
//
//	go run ./tools/newmodule -name=product -entities=product,category
//	go run ./tools/newmodule -name=product -entities=product -dry-run
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"alexGo-cloud/tools/newmodule"
)

func main() {
	var (
		name         = flag.String("name", "", "模块名（小写下蛇，必填）")
		entities     = flag.String("entities", "", "逗号分隔实体名（单数小写下蛇，必填）")
		menuPrefix   = flag.String("menu-prefix", "", "菜单/路由前缀（默认 = name）")
		dryRun       = flag.Bool("dry-run", false, "只打印计划，不落盘")
		skipFrontend = flag.Bool("skip-frontend", false, "只生成后端")
		skipBuild    = flag.Bool("skip-build", false, "跳过 go build 验证")
	)
	flag.Parse()

	var ents []string
	for _, e := range strings.Split(*entities, ",") {
		if e = strings.TrimSpace(e); e != "" {
			ents = append(ents, e)
		}
	}
	o := newmodule.Options{
		Root:         ".",
		Name:         *name,
		Entities:     ents,
		MenuPrefix:   *menuPrefix,
		DryRun:       *dryRun,
		SkipFrontend: *skipFrontend,
		VerifyBuild:  !*skipBuild && !*dryRun,
	}
	cs, err := newmodule.Run(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✘ %v\n", err)
		os.Exit(1)
	}
	fmt.Print(newmodule.BuildReport(cs, o, o.VerifyBuild))
}
