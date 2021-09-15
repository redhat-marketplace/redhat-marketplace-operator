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

	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/adminserver"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/fileretriever"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/filesender"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/database"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
	fileSender := FileSenderServer{Server: frs}

	if frs.startListener == nil {
		frs.startListener = func() (net.Listener, error) {
			return net.Listen("tcp", ":8003")
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

	defer lis.Close()

	// we're going to run the different protocol servers in parallel, so
	// make an errgroup
	var group errgroup.Group

	group.Go(func() error {
		grpcServer := grpc.NewServer()

		filesender.RegisterFileSenderServer(grpcServer, &fileSender)
		fileretriever.RegisterFileRetrieverServer(grpcServer, &fileRetriever)
		adminserver.RegisterAdminServerServer(grpcServer, &adminServer)
		reflection.Register(grpcServer)

		return grpcServer.Serve(lis)
	})

	// wait
	return group.Wait()
}
