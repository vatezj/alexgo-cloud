package newmodule

import (
	"bytes"
	"fmt"
	texttpl "text/template"

	"alexGo-cloud/pkg/codegen/model"
	"alexGo-cloud/pkg/codegen/naming"
)

// menuSeedFiles 生成 system 模块的菜单种子迁移对。
// 放 system 而非模块自身迁移：menus 表由 system 迁移创建，且现有先例
// （20261009000001_menus_vben_backfill）都在 system。
func menuSeedFiles(o *Options) ([]model.GeneratedFile, error) {
	b := menuBind{Module: o.Name, Title: naming.Pascal(o.Name)}
	for i, e := range o.Entities {
		entity := naming.EntityName(naming.Plural(e))
		b.Entities = append(b.Entities, menuEntity{
			Entity: entity,
			Snake:  naming.Snake(entity),
			Plural: naming.Plural(entity),
			Title:  naming.Pascal(entity),
			Sort:   i + 1,
		})
	}
	b.EntityCount = len(b.Entities)

	if o.SkipFrontend {
		return nil, nil // 无前端视图则不种菜单（否则菜单点开 404）
	}

	ver, err := nextMigrationVersion(fmt.Sprintf("%s/modules/system/migrations", o.Root), o.Now)
	if err != nil {
		return nil, err
	}
	base := fmt.Sprintf("modules/system/migrations/%s_%s_menus", ver, o.Name)

	var files []model.GeneratedFile
	for _, tplName := range []string{"menu_up.sql.tmpl", "menu_down.sql.tmpl"} {
		raw, err := shellFS.ReadFile("templates/" + tplName)
		if err != nil {
			return nil, err
		}
		tpl, err := texttpl.New(tplName).Parse(string(raw))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, b); err != nil {
			return nil, err
		}
		suffix := ".up.sql"
		if tplName == "menu_down.sql.tmpl" {
			suffix = ".down.sql"
		}
		files = append(files, model.GeneratedFile{Path: base + suffix, Content: buf.Bytes()})
	}
	return files, nil
}

// menuBind 菜单模板数据（独立于 shellBind，避免壳层绑定膨胀）。
type menuBind struct {
	Module      string
	Title       string
	EntityCount int
	Entities    []menuEntity
}

// menuEntity 单实体的菜单视图。
type menuEntity struct {
	Entity string
	Snake  string
	Plural string
	Title  string
	Sort   int
}
