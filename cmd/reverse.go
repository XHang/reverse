// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"xorm.io/reverse/meta"
	"xorm.io/reverse/pkg/conf"
	"xorm.io/reverse/pkg/language"
	"xorm.io/reverse/pkg/utils"
	schemas2 "xorm.io/reverse/schemas"

	"gitea.com/lunny/log"
	underscore "github.com/ahl5esoft/golang-underscore"
	"github.com/gobwas/glob"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

var (
	defaultFuncs = template.FuncMap{
		"UnTitle": utils.UnTitle,
		"Upper":   utils.UpTitle,
	}
)

func reverseFromConfig(rFile string) error {
	configs, err := conf.NewReverseConfigFromYAML(rFile)
	if err != nil {
		return err
	}

	for _, cfg := range configs {
		for _, target := range cfg.Targets {
			if err := runReverse(&cfg.Source, &target); err != nil {
				return err
			}
		}
	}

	return nil
}

// filterTables filter by target.ExcludeTables and target.IncludeTables
func filterTables(tables []*schemas.Table, target *conf.ReverseTarget) []*schemas.Table {
	var res = make([]*schemas.Table, 0, len(tables))
	underscore.Chain(tables).
		Filter(func(tbl schemas.Table, _ int) bool {
			for _, exclude := range target.ExcludeTables {
				s, _ := glob.Compile(exclude)
				if s.Match(tbl.Name) {
					return false
				}
			}

			return true
		}).
		Filter(func(tbl schemas.Table, _ int) bool {
			// if not set, all tables by default
			if len(target.IncludeTables) == 0 {
				return true
			}

			for _, include := range target.IncludeTables {
				s, _ := glob.Compile(include)
				if s.Match(tbl.Name) {
					return true
				}
			}

			return false
		}).
		Each(func(tbl schemas.Table, _ int) {
			res = append(res, &tbl)
		})

	return res
}

func runReverse(source *conf.ReverseSource, target *conf.ReverseTarget) error {
	var (
		formatter func(string) (string, error)
		importter func([]*schemas.Table) []string
	)

	orm, err := xorm.NewEngine(source.Database, source.ConnStr)
	if err != nil {
		return err
	}

	//there's a bug in xorm that it doesn't support UNSIGNED.
	//location: xorm.io\xorm@v1.3.11\dialects\mysql.go:GetColumns
	var tables []*schemas.Table
	if orm.Dialect().URI().DBType == schemas.MYSQL {
		tables, err = meta.NewLoader(orm.DB().DB, orm.Dialect().URI().DBName, target.IncludeTables...).Load()
		if err != nil {
			return err
		}
	} else {
		tables, err = orm.DBMetas()
		if err != nil {
			return err
		}
		// filter tables according includes and excludes
		tables = filterTables(tables, target)
	}
	// load configuration from language
	lang := language.GetLanguage(target.Language, target.TableName)

	// load template
	var bs []byte
	if target.Template != "" {
		bs = []byte(target.Template)
	} else if target.TemplatePath != "" {
		bs, err = ioutil.ReadFile(target.TemplatePath)
		if err != nil {
			return err
		}
	}

	var tableMapper = utils.GetMapperByName(target.TableMapper)
	var colMapper = utils.GetMapperByName(target.ColumnMapper)
	funcs := utils.MergeFuncMap(
		template.FuncMap(defaultFuncs),
		template.FuncMap{
			"TableMapper":  tableMapper.Table2Obj,
			"ColumnMapper": colMapper.Table2Obj,
		})

	if lang != nil {
		lang.BindTarget(target)

		if bs == nil {
			bs = []byte(lang.GetTemplate())
		}

		funcs = utils.MergeFuncMap(funcs, lang.GetFuncs())

		if formatter == nil {
			formatter = lang.GetFormatter()
		}

		if importter == nil {
			importter = lang.GetImportter()
		}

		target.ExtName = lang.GetExtName()
	}
	if !strings.HasPrefix(target.ExtName, ".") {
		target.ExtName = "." + target.ExtName
	}

	if bs == nil {
		return errors.New("you have to indicate template / template path or a language")
	}

	t := template.New("reverse")
	t.Funcs(funcs)

	tmpl, err := t.Parse(string(bs))
	if err != nil {
		return err
	}

	for _, table := range tables {
		if target.TablePrefix != "" {
			table.Name = strings.TrimPrefix(table.Name, target.TablePrefix)
		}
		for _, col := range table.Columns() {
			col.FieldName = colMapper.Table2Obj(col.Name)
		}
	}

	err = os.MkdirAll(target.OutputDir, os.ModePerm)
	if err != nil {
		return err
	}
	customTables := make([]*schemas2.Table, 0, len(tables))
	for _, t := range tables {
		customTables = append(customTables, &schemas2.Table{
			Table:    *t,
			DataBase: ExtractDBName(source.ConnStr),
		})
	}

	var w *os.File
	if !target.MultipleFiles {
		w, err = os.Create(filepath.Join(target.OutputDir, "models"+target.ExtName))
		if err != nil {
			return err
		}
		defer w.Close()

		imports := importter(tables)

		newbytes := bytes.NewBufferString("")
		err = tmpl.Execute(newbytes, map[string]interface{}{
			"Tables":  customTables,
			"Imports": imports,
		})
		if err != nil {
			return err
		}

		tplcontent, err := ioutil.ReadAll(newbytes)
		if err != nil {
			return err
		}
		var source string
		if formatter != nil {
			source, err = formatter(string(tplcontent))
			if err != nil {
				log.Warnf("%v", err)
				source = string(tplcontent)
			}
		} else {
			source = string(tplcontent)
		}

		w.WriteString(source)
		w.Close()
	} else {
		for _, table := range customTables {
			// imports
			tbs := []*schemas.Table{&table.Table}
			imports := importter(tbs)

			w, err := os.Create(filepath.Join(target.OutputDir, table.Name+target.ExtName))
			if err != nil {
				return err
			}
			defer w.Close()

			newbytes := bytes.NewBufferString("")
			err = tmpl.Execute(newbytes, map[string]interface{}{
				"Tables":  []*schemas2.Table{table},
				"Imports": imports,
			})
			if err != nil {
				return err
			}

			tplcontent, err := io.ReadAll(newbytes)
			if err != nil {
				return err
			}
			var source string
			if formatter != nil {
				source, err = formatter(string(tplcontent))
				if err != nil {
					log.Warnf("%v", err)
					source = string(tplcontent)
				}
			} else {
				source = string(tplcontent)
			}

			w.WriteString(source)
			w.Close()
		}
	}

	return nil
}

func ExtractDBName(dsn string) string {
	// 找到第一个 "/" 的位置
	start := strings.Index(dsn, "/")
	if start == -1 {
		return ""
	}

	// 从 "/" 后面开始找 "?"
	end := strings.Index(dsn[start+1:], "?")
	if end == -1 {
		// 没有 ?，说明数据库名到结尾
		return dsn[start+1:]
	}

	// 返回 "/" 和 "?" 之间的内容
	return dsn[start+1 : start+1+end]
}
