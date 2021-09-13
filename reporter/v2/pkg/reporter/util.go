package reporter

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	"emperror.dev/errors"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

func createTlsConfig(caCert []byte) (*tls.Config, error) {
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get system cert pool")
	}

	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		err = errors.New("failed to append cert to cert pool")
		logger.Error(err, "cert pool error")
		return nil, err
	}

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}

func newGRPCConn(
	ctx context.Context,
	address string,
	caCert []byte,
	token string,
) (*grpc.ClientConn, error) {

	options := []grpc.DialOption{}

	/* creat tls */
	tlsConf, err := createTlsConfig(caCert)
	if err != nil {
		logger.Error(err, "failed to create creds")
		return nil, err
	}

	if token != "" {
		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)))

		/* create oauth2 token  */
		oauth2Token := &oauth2.Token{
			AccessToken: token,
		}

		perRPC := oauth.NewOauthAccess(oauth2Token)

		options = append(options, grpc.WithPerRPCCredentials(perRPC))
	}

	options = append(options, grpc.WithBlock())

	return grpc.DialContext(ctx, address, options...)
}
