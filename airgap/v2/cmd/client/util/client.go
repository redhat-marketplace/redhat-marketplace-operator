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

package util

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

//InitClient Initializes gRPC client connection
func InitClient() (*grpc.ClientConn, error) {
	// Create connection
	insecure := viper.GetBool("insecure")
	var conn *grpc.ClientConn

	log, err := InitLog()
	if err != nil {
		return nil, err
	}

	// Fetch target address
	address := viper.GetString("address")
	if len(strings.TrimSpace(address)) == 0 {
		return nil, fmt.Errorf("target address is blank/empty")
	}
	log.Info("Connection credentials:", "address", address)

	if insecure {
		conn, err = grpc.Dial(address, grpc.WithInsecure())
	} else {
		cert := viper.GetString("certificate-path")
		creds, sslErr := credentials.NewClientTLSFromFile(cert, "")
		if sslErr != nil {
			return nil, fmt.Errorf("ssl error: %v", sslErr)
		}
		opts := grpc.WithTransportCredentials(creds)
		conn, err = grpc.Dial(address, opts)
	}

	// Handle any connection errors
	if err != nil {
		return nil, fmt.Errorf("connection error: %v", err)
	}
	return conn, nil
}
