package name

import (
	"strings"
	"xorm.io/xorm/names"
)

type Go struct {
	names.SnakeMapper
}

func (g Go) Table2Obj(req string) string {
	return strings.Replace(g.SnakeMapper.Table2Obj(req), "Id", "ID", -1)
}
