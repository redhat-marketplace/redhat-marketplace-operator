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

//go:build linux
// +build linux

package dqlite

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
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
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseConfig struct {
	Name         string
	Dir          string
	Url          string
	Join         *[]string
	Verbose      bool
	Log          logr.Logger
	dqliteDB     *sql.DB
	app          *app.App
	gormDB       *gorm.DB
	CACert       string
	TLSCert      string
	TLSKey       string
	CipherSuites []uint16
	MinVersion   uint16
}

// InitDB initializes the GORM connection and returns a connected struct
func (dc *DatabaseConfig) InitDB(
	cleanUpAfter time.Duration,
) (database.StoredFileStore, error) {
	err := dc.initDqlite()
	if err != nil {
		return nil, err
	}
	writer := logger.Writer(log.New(os.Stdout, "gorm-", log.LstdFlags))
	log := logger.New(writer, logger.Config{
		LogLevel: logger.Info,
	})
	dqliteDialector := dqlite.Open(dc.dqliteDB)

	db, err := gorm.Open(dqliteDialector, &gorm.Config{
		Logger: log,
	})
	if err != nil {
		return nil, err
	}

	database, _ := database.New(db, database.FileStoreConfig{CleanupAfter: cleanUpAfter})
	dc.gormDB = db
	return database, err
}

// initDqlite initializes the underlying dqlite database
func (dc *DatabaseConfig) initDqlite() error {
	dc.Dir = filepath.Join(dc.Dir, dc.Url)
	if err := os.MkdirAll(dc.Dir, 0755); err != nil {
		return errors.Wrapf(err, "can't create %s", dc.Dir)
	}
	logFunc := func(l client.LogLevel, format string, a ...interface{}) {
		if !dc.Verbose {
			return
		}
		s := fmt.Sprintf(fmt.Sprintf("%s", format), a...)
		dc.Log.Info(strings.TrimRight(s, "\n"))
	}

	options := []app.Option{app.WithAddress(dc.Url), app.WithCluster(*dc.Join), app.WithLogFunc(logFunc)}

	if len(dc.TLSCert) != 0 && len(dc.TLSKey) != 0 && len(dc.CACert) != 0 {
		tlsCert, err := tls.LoadX509KeyPair(dc.TLSCert, dc.TLSKey)
		if err != nil {
			return err
		}
		caCert, err := os.ReadFile(dc.CACert)
		if err != nil {
			return err
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)

		tlsListenConfig, tlsDialConfig := app.SimpleTLSConfig(tlsCert, pool)

		tlsListenConfig.CipherSuites = dc.CipherSuites
		tlsListenConfig.MinVersion = dc.MinVersion

		tlsDialConfig.CipherSuites = dc.CipherSuites
		tlsDialConfig.MinVersion = dc.MinVersion

		options = append(options, app.WithTLS(tlsListenConfig, tlsDialConfig))
	}

	app, err := app.New(dc.Dir, options...)
	if err != nil {
		return err
	}

	if err := app.Ready(context.Background()); err != nil {
		return err
	}

	conn, err := app.Open(context.Background(), dc.Name)
	if err != nil {
		return err
	}

	dc.app = app
	dc.dqliteDB = conn
	return conn.Ping()
}

// TryMigrate  performs database migration
func (dc *DatabaseConfig) TryMigrate() error {
	return database.Migrate(dc.gormDB)
}

// IsLeader returns true if running node is leader
func (dc *DatabaseConfig) IsLeader() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cli, err := dc.app.Leader(ctx)
	if err != nil {
		return false, err
	}
	var leader *client.NodeInfo
	for leader == nil {
		leader, err = cli.Leader(ctx)
		if err != nil {
			return false, err
		}
	}
	if leader.Address == dc.app.Address() {
		return true, nil
	}
	return false, nil
}

// Close ensures all responsibilites for the node are handled gracefully on exit
func (dc *DatabaseConfig) Close() {
	if dc != nil {
		dc.dqliteDB.Close()
		dc.app.Handover(context.Background())
		dc.app.Close()
	}
}
