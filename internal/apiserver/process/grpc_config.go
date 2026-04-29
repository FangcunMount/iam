package process

import (
	"github.com/FangcunMount/iam/internal/apiserver/config"
	"github.com/FangcunMount/iam/internal/pkg/grpc"
)

// buildGRPCServer 构建 GRPC 服务器。
func buildGRPCServer(cfg *config.Config) (*grpc.Server, error) {
	grpcConfig := grpc.NewConfig()
	if err := applyGRPCOptions(cfg, grpcConfig); err != nil {
		return nil, err
	}
	return grpcConfig.Complete().New()
}

// applyGRPCOptions 应用 GRPC 选项到配置。
func applyGRPCOptions(cfg *config.Config, grpcConfig *grpc.Config) error {
	grpcConfig.BindAddress = cfg.GRPCOptions.BindAddress
	grpcConfig.BindPort = cfg.GRPCOptions.BindPort
	grpcConfig.HealthzPort = cfg.GRPCOptions.HealthzPort

	if cfg.GRPCOptions.MTLS != nil {
		mtlsOpt := cfg.GRPCOptions.MTLS
		grpcConfig.MTLS.Enabled = mtlsOpt.Enabled
		grpcConfig.MTLS.CAFile = mtlsOpt.CAFile
		grpcConfig.MTLS.CADir = mtlsOpt.CADir
		grpcConfig.MTLS.RequireClientCert = mtlsOpt.RequireClientCert
		grpcConfig.MTLS.AllowedCNs = mtlsOpt.AllowedCNs
		grpcConfig.MTLS.AllowedOUs = mtlsOpt.AllowedOUs
		grpcConfig.MTLS.AllowedSANs = mtlsOpt.AllowedSANs
		grpcConfig.MTLS.MinTLSVersion = mtlsOpt.MinTLSVersion
		grpcConfig.MTLS.EnableAutoReload = mtlsOpt.EnableAutoReload
		if mtlsOpt.ReloadInterval > 0 {
			grpcConfig.MTLS.ReloadInterval = mtlsOpt.ReloadInterval
		}
		if mtlsOpt.CertFile != "" {
			grpcConfig.TLSCertFile = mtlsOpt.CertFile
		}
		if mtlsOpt.KeyFile != "" {
			grpcConfig.TLSKeyFile = mtlsOpt.KeyFile
		}
		if mtlsOpt.Enabled {
			grpcConfig.Insecure = false
		}
	}

	if cfg.GRPCOptions.Auth != nil {
		authOpt := cfg.GRPCOptions.Auth
		grpcConfig.Auth.Enabled = authOpt.Enabled
		grpcConfig.Auth.EnableBearer = authOpt.EnableBearer
		grpcConfig.Auth.EnableHMAC = authOpt.EnableHMAC
		grpcConfig.Auth.EnableAPIKey = authOpt.EnableAPIKey
		if authOpt.HMACTimestampValidity > 0 {
			grpcConfig.Auth.HMACTimestampValidity = authOpt.HMACTimestampValidity
		}
		grpcConfig.Auth.RequireIdentityMatch = authOpt.RequireIdentityMatch
	}

	if cfg.GRPCOptions.ACL != nil {
		aclOpt := cfg.GRPCOptions.ACL
		grpcConfig.ACL.Enabled = aclOpt.Enabled
		grpcConfig.ACL.ConfigFile = aclOpt.ConfigFile
		if aclOpt.DefaultPolicy != "" {
			grpcConfig.ACL.DefaultPolicy = aclOpt.DefaultPolicy
		}
	}

	if cfg.GRPCOptions.Audit != nil {
		grpcConfig.Audit.Enabled = cfg.GRPCOptions.Audit.Enabled
	}

	if cfg.SecureServing != nil && grpcConfig.TLSCertFile == "" && grpcConfig.TLSKeyFile == "" {
		grpcConfig.TLSCertFile = cfg.SecureServing.TLS.CertFile
		grpcConfig.TLSKeyFile = cfg.SecureServing.TLS.KeyFile
	}

	grpcConfig.Insecure = cfg.GRPCOptions.Insecure && !grpcConfig.MTLS.Enabled && grpcConfig.TLSCertFile == "" && grpcConfig.TLSKeyFile == ""
	return nil
}
