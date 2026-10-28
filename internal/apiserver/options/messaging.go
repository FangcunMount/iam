package options

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/spf13/pflag"

	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
)

// MessagingOptions selects an explicit provider. Files belong to IAM's runtime
// configuration; the SDK never discovers production credentials or resources.
type MessagingOptions struct {
	Transactions    map[string]TransactionOptions `json:"transactions,omitempty" mapstructure:"transactions"`
	Provider        string                        `json:"provider" mapstructure:"provider"`
	Endpoint        string                        `json:"endpoint" mapstructure:"endpoint"`
	CredentialsFile string                        `json:"credentials_file" mapstructure:"credentials_file"`
	CAFile          string                        `json:"ca_file" mapstructure:"ca_file"`
	RoutesFile      string                        `json:"routes_file" mapstructure:"routes_file"`
	InstanceID      string                        `json:"instance_id" mapstructure:"instance_id"`
	PublishEnabled  bool                          `json:"publish_enabled" mapstructure:"publish_enabled"`
}

func (o MessagingOptions) SelectedProvider() string {
	if o.Provider == "" {
		return "nsq"
	}
	return o.Provider
}
func (o MessagingOptions) Validate() error {
	for owner, binding := range o.Transactions {
		if o.SelectedProvider() != "rocketmq" {
			return errors.New("transactions require explicit RocketMQ provider")
		}
		if _, err := binding.ProtocolLimits(owner); err != nil {
			return err
		}
	}
	switch o.SelectedProvider() {
	case "nsq":
		return nil
	case "rocketmq":
		if o.Endpoint == "" || o.CredentialsFile == "" || o.CAFile == "" || o.RoutesFile == "" || o.InstanceID == "" {
			return errors.New("RocketMQ requires endpoint, private credential/CA/route files and stable instance_id")
		}
		return nil
	default:
		return errors.New("unsupported messaging provider")
	}
}

func (o MessagingOptions) RocketMQ(topics []string, concurrency int, timeout time.Duration) (rocketmq.Config, error) {
	if err := o.Validate(); err != nil {
		return rocketmq.Config{}, err
	}
	var credentials struct{ Username, Password string }
	body, err := os.ReadFile(o.CredentialsFile)
	if err != nil {
		return rocketmq.Config{}, errors.New("read configured RocketMQ credentials")
	}
	if json.Unmarshal(body, &credentials) != nil || credentials.Username == "" || credentials.Password == "" {
		return rocketmq.Config{}, errors.New("invalid RocketMQ credentials")
	}
	body, err = os.ReadFile(o.RoutesFile)
	if err != nil {
		return rocketmq.Config{}, errors.New("read configured RocketMQ route manifest")
	}
	manifest, err := transport.ParseRouteManifest(body)
	if err != nil {
		return rocketmq.Config{}, err
	}
	selected := make([]transport.RouteBinding, 0)
	seen := map[string]bool{}
	for _, r := range manifest.Routes {
		for _, topic := range topics {
			if r.Topic == topic {
				selected = append(selected, r)
				seen[topic] = true
				break
			}
		}
	}
	for _, topic := range topics {
		if !seen[topic] {
			return rocketmq.Config{}, errors.New("required logical topic absent from RocketMQ manifest")
		}
	}
	history := make([]transport.RouteBinding, 0)
	for _, r := range manifest.HistoricalRoutes {
		if seen[r.Topic] {
			history = append(history, r)
		}
	}
	ca, err := os.ReadFile(o.CAFile)
	if err != nil {
		return rocketmq.Config{}, errors.New("read configured RocketMQ CA")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return rocketmq.Config{}, errors.New("invalid RocketMQ CA")
	}
	return rocketmq.Config{Endpoint: o.Endpoint, AccessKey: credentials.Username, SecretKey: credentials.Password, Routes: selected, HistoricalRoutes: history, MaxInFlight: concurrency, RequestTimeout: timeout, TLS: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}, nil
}

func (o *MessagingOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Provider, "messaging.provider", o.Provider, "Explicit broker provider: nsq or rocketmq")
	fs.StringVar(&o.Endpoint, "messaging.endpoint", o.Endpoint, "RocketMQ Proxy endpoint")
	fs.StringVar(&o.CredentialsFile, "messaging.credentials_file", o.CredentialsFile, "Host-owned credential file")
	fs.StringVar(&o.CAFile, "messaging.ca_file", o.CAFile, "Host-owned RocketMQ CA file")
	fs.StringVar(&o.RoutesFile, "messaging.routes_file", o.RoutesFile, "Reviewed route manifest")
	fs.StringVar(&o.InstanceID, "messaging.instance_id", o.InstanceID, "Stable fanout instance identity")
	fs.BoolVar(&o.PublishEnabled, "messaging.publish_enabled", o.PublishEnabled, "Admit direct publishing and Relay claims")
}
