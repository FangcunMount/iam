package process

import (
	"testing"
	"time"

	apiserverconfig "github.com/FangcunMount/iam/v3/internal/apiserver/config"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	grpcpkg "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	genericoptions "github.com/FangcunMount/iam/v3/internal/pkg/options"
)

func TestApplyGRPCOptionsMapsMTLSAuthACLAudit(t *testing.T) {
	opts := apiserveroptions.NewOptions()
	opts.GRPCOptions.BindAddress = "127.0.0.9"
	opts.GRPCOptions.BindPort = 19090
	opts.GRPCOptions.HealthzPort = 19091
	opts.GRPCOptions.Insecure = true
	opts.GRPCOptions.MTLS = &genericoptions.GRPCMTLSOptions{
		Enabled:           true,
		CAFile:            "ca.pem",
		CADir:             "cas",
		CertFile:          "grpc.crt",
		KeyFile:           "grpc.key",
		RequireClientCert: false,
		AllowedCNs:        []string{"client-cn"},
		AllowedOUs:        []string{"client-ou"},
		AllowedSANs:       []string{"client.example"},
		MinTLSVersion:     "1.3",
		EnableAutoReload:  false,
		ReloadInterval:    42 * time.Second,
	}
	opts.GRPCOptions.Auth = &genericoptions.GRPCAuthOptions{
		Enabled:               true,
		EnableBearer:          false,
		EnableHMAC:            true,
		EnableAPIKey:          false,
		HMACTimestampValidity: 17 * time.Second,
		RequireIdentityMatch:  false,
	}
	opts.GRPCOptions.ACL = &genericoptions.GRPCAclOptions{
		Enabled:       true,
		ConfigFile:    "acl.yaml",
		DefaultPolicy: "allow",
	}
	opts.GRPCOptions.Audit = &genericoptions.GRPCAuditOptions{Enabled: false}

	got := grpcpkg.NewConfig()
	if err := applyGRPCOptions(&apiserverconfig.Config{Options: opts}, got); err != nil {
		t.Fatalf("applyGRPCOptions() error = %v", err)
	}

	if got.BindAddress != "127.0.0.9" || got.BindPort != 19090 || got.HealthzPort != 19091 {
		t.Fatalf("basic bind options = %s/%d/%d", got.BindAddress, got.BindPort, got.HealthzPort)
	}
	if !got.MTLS.Enabled || got.MTLS.CAFile != "ca.pem" || got.MTLS.CADir != "cas" ||
		got.TLSCertFile != "grpc.crt" || got.TLSKeyFile != "grpc.key" ||
		got.MTLS.RequireClientCert || got.MTLS.MinTLSVersion != "1.3" ||
		got.MTLS.EnableAutoReload || got.MTLS.ReloadInterval != 42*time.Second {
		t.Fatalf("mTLS options not preserved: %#v cert=%q key=%q", got.MTLS, got.TLSCertFile, got.TLSKeyFile)
	}
	if len(got.MTLS.AllowedCNs) != 1 || got.MTLS.AllowedCNs[0] != "client-cn" ||
		len(got.MTLS.AllowedOUs) != 1 || got.MTLS.AllowedOUs[0] != "client-ou" ||
		len(got.MTLS.AllowedSANs) != 1 || got.MTLS.AllowedSANs[0] != "client.example" {
		t.Fatalf("mTLS subject allowlists not preserved: %#v", got.MTLS)
	}
	if got.Insecure {
		t.Fatal("Insecure = true, want false when mTLS is enabled")
	}
	if !got.Auth.Enabled || got.Auth.EnableBearer || !got.Auth.EnableHMAC || got.Auth.EnableAPIKey ||
		got.Auth.HMACTimestampValidity != 17*time.Second || got.Auth.RequireIdentityMatch {
		t.Fatalf("auth options not preserved: %#v", got.Auth)
	}
	if !got.ACL.Enabled || got.ACL.ConfigFile != "acl.yaml" || got.ACL.DefaultPolicy != "allow" {
		t.Fatalf("ACL options not preserved: %#v", got.ACL)
	}
	if got.Audit.Enabled {
		t.Fatalf("Audit.Enabled = true, want false")
	}
}

func TestApplyGRPCOptionsFallsBackToSecureServingTLSAndControlsInsecure(t *testing.T) {
	opts := apiserveroptions.NewOptions()
	opts.GRPCOptions.Insecure = true
	opts.GRPCOptions.MTLS = nil
	opts.SecureServing.TLS.CertFile = "secure.crt"
	opts.SecureServing.TLS.KeyFile = "secure.key"

	got := grpcpkg.NewConfig()
	if err := applyGRPCOptions(&apiserverconfig.Config{Options: opts}, got); err != nil {
		t.Fatalf("applyGRPCOptions() error = %v", err)
	}
	if got.TLSCertFile != "secure.crt" || got.TLSKeyFile != "secure.key" {
		t.Fatalf("TLS fallback = %q/%q, want secure serving cert/key", got.TLSCertFile, got.TLSKeyFile)
	}
	if got.Insecure {
		t.Fatal("Insecure = true, want false when TLS cert/key are configured")
	}

	opts.SecureServing.TLS.CertFile = ""
	opts.SecureServing.TLS.KeyFile = ""
	got = grpcpkg.NewConfig()
	if err := applyGRPCOptions(&apiserverconfig.Config{Options: opts}, got); err != nil {
		t.Fatalf("applyGRPCOptions() without TLS error = %v", err)
	}
	if !got.Insecure {
		t.Fatal("Insecure = false, want true when explicitly enabled and no TLS/mTLS is configured")
	}
}
