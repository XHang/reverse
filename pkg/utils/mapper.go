package utils

import (
	"xorm.io/reverse/name"
	"xorm.io/xorm/names"
)

func GetMapperByName(mapname string) names.Mapper {
	switch mapname {
	case "gonic":
		return names.LintGonicMapper
	case "same":
		return names.SameMapper{}
	case "go":
		return name.Go{}
	default:
		return names.SnakeMapper{}
	}
}
