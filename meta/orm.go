package meta

import (
	"database/sql"
	"reflect"
	"strings"

	"xorm.io/xorm/schemas"
)

type Loader struct {
	DB        *sql.DB
	Schema    string   // 数据库名
	TableList []string // 可选：指定表名列表
}

func NewLoader(DB *sql.DB, schema string, tables ...string) *Loader {
	return &Loader{DB: DB, Schema: schema, TableList: tables}
}

// Load 增强版：只查指定表（或全部表），并构造 []*schemas.Table
func (m *Loader) Load() ([]*schemas.Table, error) {
	tables, err := m.loadTables()
	if err != nil {
		return nil, err
	}

	for _, t := range tables {
		cols, err := m.loadColumns(t)
		if err != nil {
			return nil, err
		}
		for _, col := range cols {
			t.AddColumn(col)
		}
	}

	return tables, nil
}

// 查询表列表
func (m *Loader) loadTables() ([]*schemas.Table, error) {
	var args []interface{}
	sqlStr := `
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = ?
    `
	args = append(args, m.Schema)

	if len(m.TableList) > 0 {
		sqlStr += " AND table_name IN (" + strings.Repeat("?,", len(m.TableList)-1) + "?)"
		for _, t := range m.TableList {
			args = append(args, t)
		}
	}

	rows, err := m.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*schemas.Table
	for rows.Next() {
		var name string
		rows.Scan(&name)
		result = append(result, schemas.NewTable(name, reflect.TypeOf(new(schemas.Table))))
	}
	return result, nil
}

// 查询列信息
func (m *Loader) loadColumns(t *schemas.Table) ([]*schemas.Column, error) {
	sqlStr := `
        SELECT 
            column_name,
            column_type,
            is_nullable,
            column_key,
            extra,
            column_default
        FROM information_schema.columns
        WHERE table_schema = ?
          AND table_name = ?
        ORDER BY ordinal_position
    `

	rows, err := m.DB.Query(sqlStr, m.Schema, t.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []*schemas.Column

	for rows.Next() {
		var (
			name, colType, nullable, key, extra string
			defaultVal                          sql.NullString
		)

		rows.Scan(&name, &colType, &nullable, &key, &extra, &defaultVal)

		col := &schemas.Column{
			Name:            name,
			Default:         defaultVal.String,
			Nullable:        nullable == "YES",
			IsPrimaryKey:    key == "PRI",
			IsAutoIncrement: strings.Contains(extra, "auto_increment"),
		}

		// ⭐ 关键：增强版类型解析（替代 xorm 的 ParseString）
		col.SQLType = parseMySQLType(colType)

		cols = append(cols, col)
	}

	return cols, nil
}

// ⭐ 增强版 MySQL 类型解析器（支持 UNSIGNED / JSON / datetime(3) / enum / set）
func parseMySQLType(colType string) schemas.SQLType {
	s := strings.ToUpper(colType)

	// 去掉长度，例如 VARCHAR(255)
	base := s
	if idx := strings.Index(s, "("); idx > 0 {
		base = s[:idx]
	}

	// 处理 UNSIGNED
	base = strings.TrimSpace(strings.Replace(base, "UNSIGNED", "", -1))

	switch base {
	case "INT", "INTEGER":
		return schemas.SQLType{Name: schemas.Int}
	case "BIGINT":
		return schemas.SQLType{Name: schemas.BigInt}
	case "SMALLINT":
		return schemas.SQLType{Name: schemas.SmallInt}
	case "TINYINT":
		return schemas.SQLType{Name: schemas.TinyInt}
	case "DECIMAL", "NUMERIC":
		return schemas.SQLType{Name: schemas.Decimal}
	case "FLOAT":
		return schemas.SQLType{Name: schemas.Float}
	case "DOUBLE":
		return schemas.SQLType{Name: schemas.Double}
	case "CHAR":
		return schemas.SQLType{Name: schemas.Char}
	case "VARCHAR":
		return schemas.SQLType{Name: schemas.Varchar}
	case "TEXT", "MEDIUMTEXT", "LONGTEXT":
		return schemas.SQLType{Name: schemas.Text}
	case "JSON":
		return schemas.SQLType{Name: schemas.Json}
	case "DATETIME", "TIMESTAMP":
		return schemas.SQLType{Name: schemas.DateTime}
	case "DATE":
		return schemas.SQLType{Name: schemas.Date}
	case "TIME":
		return schemas.SQLType{Name: schemas.Time}
	case "ENUM":
		return schemas.SQLType{Name: schemas.Enum}
	case "SET":
		return schemas.SQLType{Name: schemas.Set}
	default:
		return schemas.SQLType{Name: schemas.Varchar} // fallback
	}
}
