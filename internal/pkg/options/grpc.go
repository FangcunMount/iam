package options

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/pflag"
)

// GRPCOptions GRPC 服务器配置选项
type GRPCOptions struct {
	BindAddress                    string            `json:"bind_address" mapstructure:"bind-address"` // 绑定地址
	BindPort                       int               `json:"bind_port"    mapstructure:"bind-port"`    // 绑定端口
	HealthzPort                    int               `json:"healthz_port" mapstructure:"healthz-port"` // 健康检查端口
	MTLS                           *GRPCMTLSOptions  `json:"mtls"         mapstructure:"mtls"`         // mTLS 选项
	Auth                           *GRPCAuthOptions  `json:"auth"         mapstructure:"auth"`         // 应用层认证
	ACL                            *GRPCAclOptions   `json:"acl"          mapstructure:"acl"`          // ACL
	Audit                          *GRPCAuditOptions `json:"audit"        mapstructure:"audit"`        // 审计
	AuthzAssignmentConstraintsFile string            `json:"authz_assignment_constraints_file" mapstructure:"authz-assignment-constraints-file"`
	Insecure                       bool              `json:"insecure"     mapstructure:"insecure"` // 是否允许不安全（默认 true，启用 mTLS 时会强制为 false）
}

// GRPCMTLSOptions mTLS 配置
type GRPCMTLSOptions struct {
	Enabled           bool          `json:"enabled"             mapstructure:"enabled"`
	CAFile            string        `json:"ca_file"             mapstructure:"ca-file"`
	CADir             string        `json:"ca_dir"              mapstructure:"ca-dir"`
	CertFile          string        `json:"cert_file"           mapstructure:"cert-file"`
	KeyFile           string        `json:"key_file"            mapstructure:"key-file"`
	RequireClientCert bool          `json:"require_client_cert" mapstructure:"require-client-cert"`
	AllowedCNs        []string      `json:"allowed_cns"         mapstructure:"allowed-cns"`
	AllowedOUs        []string      `json:"allowed_ous"         mapstructure:"allowed-ous"`
	AllowedSANs       []string      `json:"allowed_sans"        mapstructure:"allowed-sans"`
	MinTLSVersion     string        `json:"min_tls_version"     mapstructure:"min-tls-version"`
	EnableAutoReload  bool          `json:"enable_auto_reload"  mapstructure:"enable-auto-reload"`
	ReloadInterval    time.Duration `json:"reload_interval"     mapstructure:"reload-interval"`
}

// GRPCAuthOptions 应用层认证配置
type GRPCAuthOptions struct {
	Enabled               bool          `json:"enabled"                mapstructure:"enabled"`
	EnableBearer          bool          `json:"enable_bearer"          mapstructure:"enable-bearer"`
	EnableHMAC            bool          `json:"enable_hmac"            mapstructure:"enable-hmac"`
	EnableAPIKey          bool          `json:"enable_api_key"         mapstructure:"enable-api-key"`
	HMACTimestampValidity time.Duration `json:"hmac_timestamp_validity" mapstructure:"hmac-timestamp-validity"`
	RequireIdentityMatch  bool          `json:"require_identity_match" mapstructure:"require-identity-match"`
}

// GRPCAclOptions ACL 配置
type GRPCAclOptions struct {
	Enabled       bool   `json:"enabled"        mapstructure:"enabled"`
	ConfigFile    string `json:"config_file"    mapstructure:"config-file"`
	DefaultPolicy string `json:"default_policy" mapstructure:"default-policy"`
}

// GRPCAuditOptions 审计配置
type GRPCAuditOptions struct {
	Enabled bool `json:"enabled" mapstructure:"enabled"`
}

// NewGRPCOptions 创建默认的 GRPC 配置选项
func NewGRPCOptions() *GRPCOptions {
	return &GRPCOptions{
		BindAddress: "127.0.0.1",
		BindPort:    9090,
		HealthzPort: 9091,
		Insecure:    true,
		MTLS: &GRPCMTLSOptions{
			Enabled:           false,
			RequireClientCert: true,
			MinTLSVersion:     "1.2",
			EnableAutoReload:  true,
			ReloadInterval:    5 * time.Minute,
		},
		Auth: &GRPCAuthOptions{
			Enabled:               false,
			EnableBearer:          true,
			EnableHMAC:            true,
			EnableAPIKey:          true,
			HMACTimestampValidity: 5 * time.Minute,
			RequireIdentityMatch:  true,
		},
		ACL: &GRPCAclOptions{
			Enabled:       false,
			DefaultPolicy: "deny",
		},
		Audit: &GRPCAuditOptions{
			Enabled: true,
		},
	}
}

// Validate 验证GRPCOptions
func (s *GRPCOptions) Validate() []error {
	var errors []error

	if s.BindPort < 0 || s.BindPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--grpc.bind-port %v must be between 0 and 65535, inclusive. 0 for turning off insecure (HTTP) port",
				s.BindPort,
			),
		)
	}

	return errors
}

// AddFlags 添加命令行参数
func (s *GRPCOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindAddress, "grpc.bind-address", s.BindAddress, ""+
		"The IP address on which to serve the --grpc.bind-port(set to 0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces).")

	fs.IntVar(&s.BindPort, "grpc.bind-port", s.BindPort, ""+
		"The port on which to serve unsecured, unauthenticated grpc access. It is assumed "+
		"that firewall rules are set up such that this port is not reachable from outside of "+
		"the deployed machine and that port 443 on the iam public address is proxied to this "+
		"port. This is performed by nginx in the default setup. Set to zero to disable.")

	fs.IntVar(&s.HealthzPort, "grpc.healthz-port", s.HealthzPort, ""+
		"The port on which to serve grpc health check.")
}

// ApplyTo 应用配置到服务器
func (s *GRPCOptions) ApplyTo(c *GRPCConfig) error {
	c.Addr = net.JoinHostPort(s.BindAddress, fmt.Sprintf("%d", s.BindPort))
	c.HealthzAddr = net.JoinHostPort(s.BindAddress, fmt.Sprintf("%d", s.HealthzPort))

	// mTLS 配置
	if s.MTLS != nil {
		c.MTLS.Enabled = s.MTLS.Enabled
		c.MTLS.CAFile = s.MTLS.CAFile
		c.MTLS.CADir = s.MTLS.CADir
		c.MTLS.RequireClientCert = s.MTLS.RequireClientCert
		c.MTLS.AllowedCNs = s.MTLS.AllowedCNs
		c.MTLS.AllowedOUs = s.MTLS.AllowedOUs
		c.MTLS.AllowedSANs = s.MTLS.AllowedSANs
		c.MTLS.MinTLSVersion = s.MTLS.MinTLSVersion
		c.MTLS.EnableAutoReload = s.MTLS.EnableAutoReload
		if s.MTLS.ReloadInterval > 0 {
			c.MTLS.ReloadInterval = s.MTLS.ReloadInterval
		}
		// 优先使用 mTLS 的证书文件
		if s.MTLS.CertFile != "" {
			c.TLSCertFile = s.MTLS.CertFile
		}
		if s.MTLS.KeyFile != "" {
			c.TLSKeyFile = s.MTLS.KeyFile
		}
		// 启用 mTLS 时强制关闭不安全模式
		if s.MTLS.Enabled {
			c.Insecure = false
		}
	}

	// 应用层认证
	if s.Auth != nil {
		c.Auth.Enabled = s.Auth.Enabled
		c.Auth.EnableBearer = s.Auth.EnableBearer
		c.Auth.EnableHMAC = s.Auth.EnableHMAC
		c.Auth.EnableAPIKey = s.Auth.EnableAPIKey
		if s.Auth.HMACTimestampValidity > 0 {
			c.Auth.HMACTimestampValidity = s.Auth.HMACTimestampValidity
		}
		c.Auth.RequireIdentityMatch = s.Auth.RequireIdentityMatch
	}

	// ACL
	if s.ACL != nil {
		c.ACL.Enabled = s.ACL.Enabled
		c.ACL.ConfigFile = s.ACL.ConfigFile
		if s.ACL.DefaultPolicy != "" {
			c.ACL.DefaultPolicy = s.ACL.DefaultPolicy
		}
	}

	// 审计
	if s.Audit != nil {
		c.Audit.Enabled = s.Audit.Enabled
	}

	// 如果未启用 mTLS 但提供了服务端证书，则仍然走 TLS（禁用 Insecure）
	if !s.Insecure && (s.MTLS != nil && (s.MTLS.CertFile != "" && s.MTLS.KeyFile != "")) {
		c.Insecure = false
	} else {
		c.Insecure = s.Insecure
	}

	return nil
}
