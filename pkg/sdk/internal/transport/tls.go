package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/FangcunMount/iam/pkg/sdk/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// BuildTLSCredentials 构建 TLS 凭证
func BuildTLSCredentials(tlsCfg *config.TLSConfig) (grpc.DialOption, error) {
	if tlsCfg == nil || !tlsCfg.Enabled {
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}

	tlsConfig, err := BuildTLSConfig(tlsCfg)
	if err != nil {
		return nil, err
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

// BuildTLSConfig 构建 tls.Config
func BuildTLSConfig(cfg *config.TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
		MinVersion:         cfg.MinVersion,
	}

	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	if len(cfg.CACertPEM) > 0 || cfg.CACert != "" {
		certPool := x509.NewCertPool()

		var caCert []byte
		if len(cfg.CACertPEM) > 0 {
			caCert = cfg.CACertPEM
		} else {
			var err error
			caCert, err = os.ReadFile(cfg.CACert)
			if err != nil {
				return nil, fmt.Errorf("read CA cert: %w", err)
			}
		}

		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = certPool
	}

	if (len(cfg.ClientCertPEM) > 0 && len(cfg.ClientKeyPEM) > 0) ||
		(cfg.ClientCert != "" && cfg.ClientKey != "") {
		var (
			cert tls.Certificate
			err  error
		)

		if len(cfg.ClientCertPEM) > 0 {
			cert, err = tls.X509KeyPair(cfg.ClientCertPEM, cfg.ClientKeyPEM)
		} else {
			cert, err = tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		}
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
