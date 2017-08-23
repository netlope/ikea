package ikea

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/davecgh/go-spew/spew"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

/*
 * database
 */

type Database struct {
	tables tables
}

func (db *Database) Get(n string) rows {

	for _, t := range db.tables {
		if t.Name == n {
			return t.rows
		}
	}

	return rows{}
}

func isStringInSlice(s string, i []string) bool {
	for _, v := range i {
		if s == v {
			return true
		}
	}
	return false
}

func (db *Database) Tables() []string {

	var tables []string
	for _, t := range db.tables {
		tables = append(tables, t.getChildTables()...)
		tables = append(tables, t.Name)
	}

	return tables
}

func (t table) getChildTables() []string {

	var tables []string
	for _, r := range t.rows {

		for _, t := range r.childs {
			tables = append(tables, t.getChildTables()...)

			if !isStringInSlice(t.Name, tables) {
				tables = append(tables, t.Name)
			}
		}

	}

	return tables

}

/*
 * fields
 */

type field struct {
	key   string
	value interface{}
}

func (f *field) parseValue() string {

	if strings.ToLower(f.key) == "password" {
		password := []byte(f.value.(string))
		hash, _ := bcrypt.GenerateFromPassword(password, bcrypt.MinCost)
		return fmt.Sprintf("%s", hash)
	}

	return f.value.(string)
}

/*
 * row(s)
 */

type row struct {
	fields []field
	childs []table
}

type rows []row

func (r rows) GetChildRowsByName(n string) rows {
	var rows rows

	for _, row := range r {

		for _, child := range row.childs {

			if child.Name != n {
				continue
			}

			rows = append(rows, child.rows...)

		}

	}

	return rows
}

func (r rows) GetChildTableByName(n string) table {

	var t table

	for _, row := range r {

		for _, child := range row.childs {

			if child.Name != n {
				continue
			}

			if t.Name == "" {
				t.Name = child.Name
				t.ParentName = child.ParentName
			}

			t.rows = append(t.rows, child.rows...)

		}

	}
	return t
}

// @TODO the filter is not subtractiv but it should
func (r rows) Filter(filters ...Filter) rows {

	if filters == nil {
		return r
	}

	var rows rows
	for _, row := range r {

		matches := 0

		for _, v := range row.fields {

			for _, filter := range filters {

				if filter.Key == v.key && filter.Value == v.value {
					matches++
				}
			}

		}

		if matches == len(filters) {
			rows = append(rows, row)
		}

	}
	return rows
}

func (r *row) get(k string) string {
	for _, v := range r.fields {
		if v.key == k {
			return v.value.(string)
		}
	}
	return ""
}

func (r *row) GetChildRowsByTableName(tableName string) rows {

	for _, v := range r.childs {
		if v.Name == tableName {
			return v.rows
		}
	}
	return rows{}
}

func (r *row) Child(tableName string) table {

	for _, v := range r.childs {
		if v.Name == tableName {
			return v
		}
	}
	return table{}
}

func (r *row) addField(k string, v interface{}) {
	f := field{k, v}
	r.fields = append(r.fields, f)
}

func (r *row) toString(tableName string) string {

	if len(r.fields) == 0 {
		return ""
	}

	var queryKeys []string
	var queryValues string

	for _, v := range r.fields {

		if v.key == "invalid" && v.value == true {
			return ""
		}

		switch reflect.ValueOf(v.value).Kind() {
		case reflect.Int:
			queryValues += fmt.Sprintf("%d, ", v.value)

		case reflect.Bool:
			queryValues += fmt.Sprintf("%t, ", v.value)

		default:
			queryValues += fmt.Sprintf("'%s', ", v.parseValue())

		}

		queryKeys = append(queryKeys, v.key)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(queryKeys, ", "),
		queryValues[:len(queryValues)-2])

	return query

}

func (r rows) ToStructSlice(s interface{}) {
	value := reflect.Indirect(reflect.ValueOf(s))
	valueType := value.Type().Elem()
	for _, v := range r {
		valueNew := reflect.New(valueType)
		v.ToStruct(valueNew.Interface())
		value = reflect.Append(value, reflect.Indirect(valueNew))
	}
	reflect.Indirect(reflect.ValueOf(s)).Set(value)
}

func (r *row) ToStruct(s interface{}) {

	sValue := reflect.Indirect(reflect.ValueOf(s))
	sFields := make(map[string]reflect.Value)

	for i := 0; i < sValue.NumField(); i++ {

		switch sValue.Field(i).Kind() {
		case reflect.Struct:

			for ii := 0; ii < sValue.Field(i).NumField(); ii++ {
				if sValue.Field(i).Field(ii).CanSet() {
					name := strings.ToLower(sValue.Field(i).Type().Field(ii).Name)
					sFields[name] = sValue.Field(i).Field(ii)
				}
			}

		default:
			if sValue.Field(i).CanSet() {
				name := strings.ToLower(sValue.Type().Field(i).Name)
				sFields[name] = sValue.Field(i)
			}
		}

	}

	for _, v := range r.fields {

		k, exists := sFields[v.key]
		if !exists {
			continue
		}

		if k.Type() == reflect.TypeOf(v.value) {
			k.Set(reflect.ValueOf(v.value))
		}

	}

}

/*
 * table
 */

type table struct {
	Name       string
	ParentName string
	rows       []row
}

type tables []table

func (t *table) crawl(m interface{}) {
	switch reflect.ValueOf(m).Kind() {
	case reflect.Map:

		var r row
		for k, v := range m.(map[interface{}]interface{}) {

			switch reflect.ValueOf(v).Kind() {
			case reflect.String, reflect.Int, reflect.Bool:

				if v == ":UUIDv4" {
					v = fmt.Sprintf("%s", uuid.NewV4())
				}

				r.addField(k.(string), v)

			default:

				if t.rows == nil && t.Name == "" {
					t.Name = k.(string)
					t.crawl(v)
				} else {
					table := table{
						Name:       k.(string),
						ParentName: t.Name,
					}
					table.crawl(v)
					r.childs = append(r.childs, table)
				}

			}

		}
		t.rows = append(t.rows, r)

	case reflect.Slice:
		for _, v := range m.([]interface{}) {
			t.crawl(v)
		}

	default:
		spew.Dump(m)
	}

}

func (t *table) walk(p row) []string {

	var insert string
	var inserts []string

	for _, row := range t.rows {

		if p.childs != nil && t.ParentName != "" {
			row.addField(t.ParentName[:len(t.ParentName)-1]+"_id", p.get("id"))
		}

		insert = row.toString(t.Name)

		if insert != "" {
			inserts = append(inserts, insert)

			for _, child := range row.childs {
				inserts = append(inserts, child.walk(row)...)
			}

		}

	}

	return inserts
}

/*
 * filter
 */

type Filter struct {
	Key   string
	Value interface{}
}

func (db *Database) Populate(m interface{}) {
	switch reflect.ValueOf(m).Kind() {

	case reflect.Map:

		for k, v := range m.(map[string]interface{}) {

			t := table{Name: k}
			t.crawl(v)

			db.tables = append(db.tables, t)

		}

	default:
		fmt.Printf("%-v", m)

	}

}

func (db *Database) GenerateInserts() []string {
	var inserts []string

	for _, t := range db.tables {
		inserts = append(inserts, t.walk(row{})...)
	}

	return inserts
}

func (db *Database) Load(filename string) error {

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	fixtures := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(file), &fixtures)
	if err != nil {
		return err
	}

	db.Populate(fixtures)
	return nil
}
