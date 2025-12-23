package schemas

import "xorm.io/xorm/schemas"

type Table struct {
	schemas.Table
	DataBase string
}
