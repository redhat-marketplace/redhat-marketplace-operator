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

package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/canonical/go-dqlite/app"
	"github.com/canonical/go-dqlite/client"
	"github.com/pkg/errors"
)

type Database struct {
	db  *sql.DB
	app *app.App
}

func InitDB(dir string, url string, join *[]string, verbose bool) (*Database, error) {
	dir = filepath.Join(dir, url)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "can't create %s", dir)
	}
	logFunc := func(l client.LogLevel, format string, a ...interface{}) {
		if !verbose {
			return
		}
		log.Printf(fmt.Sprintf("%s: %s\n", l.String(), format), a...)
	}

	app, err := app.New(dir, app.WithAddress(url), app.WithCluster(*join), app.WithLogFunc(logFunc))
	if err != nil {
		return nil, err
	}

	if err := app.Ready(context.Background()); err != nil {
		return nil, err
	}

	db, err := app.Open(context.Background(), "database")
	if err != nil {
		return nil, err
	}

	return &Database{db: db, app: app}, db.Ping()
}

func (d *Database) Close() {
	if d != nil {
		d.db.Close()
		d.app.Handover(context.Background())
		d.app.Close()
	}
}
