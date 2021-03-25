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
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/app"
	"github.com/canonical/go-dqlite/client"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	dqlite "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/dqlite/driver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"gorm.io/gorm"
)

type DatabaseConfig struct {
	Name     string
	Dir      string
	Url      string
	Join     *[]string
	Verbose  bool
	Log      logr.Logger
	dqliteDB *sql.DB
	app      *app.App
	gormDB   *gorm.DB
}

// Initialize the GORM connection and return connected struct
func (dc *DatabaseConfig) InitDB() (*database.Database, error) {
	database, err := dc.initDqlite()
	if err != nil {
		return nil, err
	}

	dqliteDialector := dqlite.Open(database.SqlDB)
	database.DB, err = gorm.Open(dqliteDialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	database.Log = dc.Log
	dc.gormDB = database.DB
	return database, err
}

// Initialize the underlying dqlite database and populate a *Database object with the dqlite connection and app
func (dc *DatabaseConfig) initDqlite() (*database.Database, error) {
	dc.Dir = filepath.Join(dc.Dir, dc.Url)
	if err := os.MkdirAll(dc.Dir, 0755); err != nil {
		return nil, errors.Wrapf(err, "can't create %s", dc.Dir)
	}
	logFunc := func(l client.LogLevel, format string, a ...interface{}) {
		if !dc.Verbose {
			return
		}
		s := fmt.Sprintf(fmt.Sprintf("%s", format), a...)
		dc.Log.Info(strings.TrimRight(s, "\n"))
	}

	app, err := app.New(dc.Dir, app.WithAddress(dc.Url), app.WithCluster(*dc.Join), app.WithLogFunc(logFunc))
	if err != nil {
		return nil, err
	}

	if err := app.Ready(context.Background()); err != nil {
		return nil, err
	}

	conn, err := app.Open(context.Background(), dc.Name)
	if err != nil {
		return nil, err
	}

	dc.app = app
	dc.dqliteDB = conn
	return &database.Database{SqlDB: conn}, conn.Ping()
}

func (dc *DatabaseConfig) TryMigrate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cli, err := dc.app.Leader(ctx)
	if err != nil {
		dc.Log.Error(err, "Could not find leader for migration")
		return err
	}

	dc.Log.Info("Verifying leadership before migration")
	var leader *client.NodeInfo
	for leader == nil {
		leader, err = cli.Leader(ctx)
		if err != nil {
			dc.Log.Error(err, "Could not find leader for migration")
			return err
		}
	}
	dc.Log.Info("Leader elected")

	if leader.Address != dc.app.Address() {
		return nil
	} else if dc.gormDB != nil {
		dc.Log.Info("Performing migration")
		return dc.gormDB.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	} else {
		return errors.New("GORM connection has not initialised: Connection of type *gorm.DB is nil")
	}
}

func (dc *DatabaseConfig) Close() {
	if dc != nil {
		dc.dqliteDB.Close()
		dc.app.Handover(context.Background())
		dc.app.Close()
	}
}
