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
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewGRPCClient(ctx context.Context, lis *bufconn.Listener) *grpc.ClientConn {
	conn, _ := grpc.DialContext(ctx, "", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithInsecure())

	return conn
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
