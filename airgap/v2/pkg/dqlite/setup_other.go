//go:build !linux
// +build !linux

package dqlite

import (
	"io"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DatabaseConfig struct {
	Name    string
	Dir     string
	Url     string
	Join    *[]string
	Verbose bool
	Log     logr.Logger
	gormDB  *gorm.DB
	CACert  string
	TLSCert string
	TLSKey  string

	closer io.Closer
}

// InitDB initializes the GORM connection and returns a connected struct
func (dc *DatabaseConfig) InitDB(
	cleanUpAfter time.Duration,
) (database.StoredFileStore, error) {
	dc.Log.Info("WARNING ---- running sqlite mode because os is not linux ---- WARNING")
	p := filepath.Join(dc.Dir, dc.Name)
	db, err := gorm.Open(sqlite.Open(p), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	database, closer := database.New(db, database.FileStoreConfig{CleanupAfter: cleanUpAfter})
	dc.gormDB = db
	dc.closer = closer
	return database, err
}

// TryMigrate  performs database migration
func (dc *DatabaseConfig) TryMigrate() error {
	return database.Migrate(dc.gormDB)
}

// IsLeader returns true if running node is leader
func (dc *DatabaseConfig) IsLeader() (bool, error) {
	dc.Log.Info("WARNING ---- running sqlite mode because os is not linux ---- WARNING")
	return true, nil
}

// Close ensures all responsibilites for the node are handled gracefully on exit
func (dc *DatabaseConfig) Close() {
	if dc != nil {
		dc.closer.Close()
	}
}
