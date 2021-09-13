package clienttest

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/internal/server"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"storj.io/drpc"
	"storj.io/drpc/drpcconn"
	"storj.io/drpc/drpcmigrate"
)

func NewDrpcClient(lis *bufconn.Listener) drpc.Conn {
	rawconn, err := lis.Dial()
	if err != nil {
		panic(err)
	}

	return drpcconn.New(drpcmigrate.NewHeaderConn(rawconn, drpcmigrate.DRPCHeader))
}

func SetupServer(
	t *testing.T,
	ctx context.Context,
	lis net.Listener,
	db *database.Database,
	dbName string,
) {
	//Initialize logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize zapr, due to error: %v", err))
	}
	log := zapr.NewLogger(zapLog)

	//Create Sqlite Database
	gormDb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("Error during creation of Database")
	}
	db.DB = gormDb
	db.Log = log

	//Create tables
	err = db.DB.AutoMigrate(&models.FileMetadata{}, &models.File{}, &models.Metadata{})
	if err != nil {
		t.Fatalf("Error during creation of Models: %v", err)
	}

	bs := server.WithCustomListener(&server.Server{FileStore: db, Log: log}, func() (net.Listener, error) {
		return lis, nil
	})

	go func() {
		bs.Start(ctx)
		db.Close()
		lis.Close()
	}()
}
