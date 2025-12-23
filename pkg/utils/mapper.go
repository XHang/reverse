package utils

import (
	a "xorm.io/reverse/names"
	"xorm.io/xorm/names"
)

func GetMapperByName(mapname string) names.Mapper {
	switch mapname {
	case "gonic":
		return names.LintGonicMapper
	case "same":
		return names.SameMapper{}
	case "go":
		return a.Go{}
	default:
		return names.SnakeMapper{}
	}
}
