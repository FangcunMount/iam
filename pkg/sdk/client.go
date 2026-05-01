package sdk

import (
	"context"
	"fmt"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	idpv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/idp/v2"
	authclient "github.com/FangcunMount/iam/v2/pkg/sdk/auth/client"
	"github.com/FangcunMount/iam/v2/pkg/sdk/authz"
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
	"github.com/FangcunMount/iam/v2/pkg/sdk/identity"
	"github.com/FangcunMount/iam/v2/pkg/sdk/idp"
	internaltransport "github.com/FangcunMount/iam/v2/pkg/sdk/internal/transport"
	"google.golang.org/grpc"
)

// Client IAM 统一客户端。
type Client struct {
	conn *grpc.ClientConn
	cfg  *Config

	authClient        *authclient.Client
	authzClient       *authz.Client
	identityClient    *identity.Client
	profileLinkClient *identity.ProfileLinkClient
	idpClient         *idp.Client
}

// NewClient 创建 IAM 客户端。
func NewClient(ctx context.Context, cfg *Config, opts ...ClientOption) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("sdk: config is required")
	}

	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	clientOpts := config.ApplyOptions(opts...)

	if len(cfg.Metadata) > 0 {
		clientOpts.UnaryInterceptors = append(
			clientOpts.UnaryInterceptors,
			internaltransport.MetadataInterceptor(cfg.Metadata),
		)
	}

	conn, err := internaltransport.Dial(ctx, cfg, clientOpts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		conn: conn,
		cfg:  cfg,
	}
	client.initSubClients()
	return client, nil
}

func (c *Client) initSubClients() {
	authService := authnv2.NewAuthServiceClient(c.conn)
	accountOnboardingService := authnv2.NewAccountOnboardingServiceClient(c.conn)
	jwksService := authnv2.NewJWKSServiceClient(c.conn)
	c.authClient = authclient.NewClient(authService, accountOnboardingService, jwksService)

	authorizationService := authzv2.NewAuthorizationServiceClient(c.conn)
	c.authzClient = authz.NewClient(authorizationService)

	readService := identityv2.NewIdentityReadClient(c.conn)
	lifecycleService := identityv2.NewIdentityLifecycleClient(c.conn)
	c.identityClient = identity.NewClient(readService, lifecycleService)

	queryService := identityv2.NewProfileLinkQueryClient(c.conn)
	commandService := identityv2.NewProfileLinkCommandClient(c.conn)
	c.profileLinkClient = identity.NewProfileLinkClient(queryService, commandService)

	idpService := idpv2.NewIDPServiceClient(c.conn)
	c.idpClient = idp.NewClient(idpService)
}

func (c *Client) Auth() *authclient.Client {
	return c.authClient
}

func (c *Client) Authz() *authz.Client {
	return c.authzClient
}

func (c *Client) Identity() *identity.Client {
	return c.identityClient
}

func (c *Client) ProfileLink() *identity.ProfileLinkClient {
	return c.profileLinkClient
}

func (c *Client) IDP() *idp.Client {
	return c.idpClient
}

func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
