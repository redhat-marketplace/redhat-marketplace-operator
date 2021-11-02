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

package server

import (
	"context"
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	Log       logr.Logger
	FileStore database.StoredFileStore

	APIEndpoint     string
	GatewayEndpoint string

	Health                  *health.Server
	APIListenerProvider     func(addr string) (net.Listener, error)
	GatewayListenerProvider func(addr string) (net.Listener, error)
}

func (frs *Server) Start(ctx context.Context) error {
	healthServer := health.NewServer()
	fs := &FileServer{Server: frs, Health: healthServer}

	defaultListener := func(addr string) (net.Listener, error) {
		return net.Listen("tcp", addr)
	}

	if frs.APIListenerProvider == nil {
		frs.APIListenerProvider = defaultListener
	}

	if frs.GatewayListenerProvider == nil {
		frs.GatewayListenerProvider = defaultListener
	}

	// we're going to run the different protocol servers in parallel, so
	// make an errgroup
	var group errgroup.Group

	group.Go(func() error {
		lis, err := frs.APIListenerProvider(frs.APIEndpoint)

		if err != nil {
			return err
		}

		grpcServer := grpc.NewServer()

		fileserver.RegisterFileServerServer(grpcServer, fs)
		grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
		reflection.Register(grpcServer)

		return grpcServer.Serve(lis)
	})

	group.Go(func() error {
		lis, err := frs.GatewayListenerProvider(frs.GatewayEndpoint)

		if err != nil {
			return err
		}

		mux := runtime.NewServeMux()
		opts := []grpc.DialOption{grpc.WithInsecure()}

		err = fileserver.RegisterFileServerHandlerFromEndpoint(ctx, mux, frs.APIEndpoint, opts)
		if err != nil {
			return err
		}

		err = fs.RegisterHTTPRoutes(mux)
		if err != nil {
			return err
		}

		// Start HTTP server (and proxy calls to gRPC server endpoint)
		return http.Serve(lis, mux)
	})

	// wait
	return group.Wait()
}
