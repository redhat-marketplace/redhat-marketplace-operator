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
	"crypto/tls"
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/adminserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"storj.io/drpc/drpchttp"
	"storj.io/drpc/drpcmigrate"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

type Server struct {
	Log       logr.Logger
	FileStore database.FileStore

	TLSKey, TLSCert string

	startListener func() (net.Listener, error)
	tlsListener   func(net.Listener) (net.Listener, error)
}

func WithAddress(s *Server, address string) *Server {
	s.startListener = func() (net.Listener, error) {
		return net.Listen("tcp", address)
	}
	return s
}

func WithTLS(s *Server, tlsKey, tlsCert string) *Server {
	s.tlsListener = func(l net.Listener) (net.Listener, error) {
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)

		if err != nil {
			return nil, err
		}

		return tls.NewListener(l, &tls.Config{
			Certificates: []tls.Certificate{
				cert,
			},
		}), nil
	}
	return s
}

func WithCustomListener(s *Server, listen func() (net.Listener, error)) *Server {
	s.startListener = listen
	return s
}

func (frs *Server) Start(ctx context.Context) error {
	adminServer := AdminServer{Server: frs}
	fileRetriever := FileRetrieverServer{Server: frs}
	drpcFileRetriever := DRPCFileRetrieverServer{FileRetrieverServer: &fileRetriever}
	fileSender := FileSenderServer{Server: frs}
	drpcFileSender := DRPCFileSenderServer{FileSenderServer: &fileSender}

	if frs.startListener == nil {
		frs.startListener = func() (net.Listener, error) {
			return net.Listen("tcp", ":8001")
		}
	}

	if frs.tlsListener == nil {
		frs.tlsListener = func(in net.Listener) (net.Listener, error) {
			return in, nil
		}
	}

	// listen on a tcp socket
	lis, err := frs.startListener()
	if err != nil {
		return err
	}

	// create a listen mux that evalutes enough bytes to recognize the DRPC header
	lisMux := drpcmigrate.NewListenMux(lis, len(drpcmigrate.DRPCHeader))

	// we're going to run the different protocol servers in parallel, so
	// make an errgroup
	var group errgroup.Group

	// create a drpc RPC mux
	m := drpcmux.New()

	group.Go(func() error {
		lis, err := net.Listen("tcp", ":8003")

		if err != nil {
			return err
		}

		grpcServer := grpc.NewServer()

		filesender.RegisterFileSenderServer(grpcServer, &fileSender)
		fileretriever.RegisterFileRetrieverServer(grpcServer, &fileRetriever)
		adminserver.RegisterAdminServerServer(grpcServer, &adminServer)
		reflection.Register(grpcServer)

		return grpcServer.Serve(lis)
	})

	// drpc handling
	group.Go(func() error {
		err = filesender.DRPCRegisterFileSender(m, &drpcFileSender)
		if err != nil {
			return err
		}

		err = fileretriever.DRPCRegisterFileRetriever(m, &drpcFileRetriever)
		if err != nil {
			return err
		}

		err = adminserver.DRPCRegisterAdminServer(m, &adminServer)
		if err != nil {
			return err
		}

		// register the proto-specific methods on the mux
		// create a drpc server
		s := drpcserver.New(m)

		// grap the listen mux route for the DRPC Header
		drpcLis := lisMux.Route(drpcmigrate.DRPCHeader)
		drpcLis, err := frs.tlsListener(drpcLis)

		if err != nil {
			return err
		}

		// run the server
		// N.B.: if you want TLS, you need to wrap the drpcLis net.Listener
		// with TLS before passing to Serve here.
		return s.Serve(ctx, drpcLis)
	})

	// http handling
	group.Go(func() error {
		// create an http server using the drpc mux wrapped in a handler
		s := http.Server{Handler: drpchttp.New(m)}
		htmlLis, err := frs.tlsListener(lisMux.Default())

		if err != nil {
			return err
		}

		// run the server
		return s.Serve(htmlLis)
	})

	// run the listen mux
	group.Go(func() error {
		return lisMux.Run(ctx)
	})

	// wait
	return group.Wait()
}
