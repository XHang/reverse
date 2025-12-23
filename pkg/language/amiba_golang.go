// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package language

import "xorm.io/xorm/schemas"

type AmibaGolang struct {
	GoLanguage
}

func (g *AmibaGolang) GetName() string {
	return "AmibaGolang"
}

func (g *AmibaGolang) GetImportter() func([]*schemas.Table) []string {
	return func(tables []*schemas.Table) []string {
		imports := g.GenGoImports(tables)
		imports = append([]string{"amiba.fun/amiba/go-component/orm"}, imports...)
		return imports
	}
}
func NewAmibaGolang() *AmibaGolang {
	return &AmibaGolang{
		GoLanguage: *NewGoLanguage(),
	}
}
