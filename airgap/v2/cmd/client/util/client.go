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
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

// InitClient Initializes gRPC client connection
func InitClient() (*grpc.ClientConn, error) {
	// Create connection
	insecure := viper.GetBool("insecure")
	var conn *grpc.ClientConn
	var err error

	// Fetch target address
	address := viper.GetString("address")
	if len(strings.TrimSpace(address)) == 0 {
		return nil, fmt.Errorf("target address is blank/empty")
	}

	options := []grpc.DialOption{}

	if insecure {
		options = append(options, grpc.WithInsecure())
	} else {
		cert := viper.GetString("certificate-path")
		creds, sslErr := credentials.NewClientTLSFromFile(cert, "")
		if sslErr != nil {
			return nil, fmt.Errorf("ssl error: %v", sslErr)
		}
		options = append(options, grpc.WithTransportCredentials(creds))

		tokenString := viper.GetString("token")
		if len(tokenString) != 0 {
			oauth2Token := &oauth2.Token{
				AccessToken: tokenString,
			}
			perRPC := oauth.NewOauthAccess(oauth2Token)
			options = append(options, grpc.WithPerRPCCredentials(perRPC))
		}
	}

	conn, err = grpc.Dial(address, options...)

	// Handle any connection errors
	if err != nil {
		return nil, fmt.Errorf("connection error: %v", err)
	}
	return conn, nil
}
