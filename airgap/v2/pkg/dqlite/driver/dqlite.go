// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dqlite

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// DriverName is the default driver name for DQLite.
const DriverName = "dqlite"

type Dialector struct {
	db        *sql.DB
	dialector *sqlite.Dialector
}

func Open(db *sql.DB) gorm.Dialector {
	return &Dialector{db: db, dialector: &sqlite.Dialector{}}
}

func (wrapper Dialector) Name() string {
	return DriverName
}

func (wrapper Dialector) Initialize(db *gorm.DB) (err error) {
	if wrapper.dialector.DriverName == "" {
		wrapper.dialector.DriverName = DriverName
	}

	// register callbacks
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
		LastInsertIDReversed: true,
	})

	if wrapper.dialector.Conn != nil {
		db.ConnPool = wrapper.dialector.Conn
	} else {
		db.ConnPool = wrapper.db
	}

	for k, v := range wrapper.dialector.ClauseBuilders() {
		db.ClauseBuilders[k] = v
	}
	return
}

func (wrapper Dialector) ClauseBuilders() map[string]clause.ClauseBuilder {
	return wrapper.dialector.ClauseBuilders()
}

func (wrapper Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return wrapper.dialector.DefaultValueOf(field)
}

func (wrapper Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return wrapper.dialector.Migrator(db)
}

func (wrapper Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	wrapper.dialector.BindVarTo(writer, stmt, v)
}

func (wrapper Dialector) QuoteTo(writer clause.Writer, str string) {
	wrapper.dialector.QuoteTo(writer, str)
}

func (wrapper Dialector) Explain(sql string, vars ...interface{}) string {
	return wrapper.dialector.Explain(sql, vars...)
}

func (wrapper Dialector) DataTypeOf(field *schema.Field) string {
	return wrapper.dialector.DataTypeOf(field)
}

func (wrapper Dialector) SavePoint(tx *gorm.DB, name string) error {
	return wrapper.dialector.SavePoint(tx, name)
}

func (wrapper Dialector) RollbackTo(tx *gorm.DB, name string) error {
	return wrapper.dialector.RollbackTo(tx, name)
}
