package sdk

import (
	"context"
	"fmt"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	authzv1 "github.com/FangcunMount/iam/api/grpc/iam/authz/v1"
	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	idpv1 "github.com/FangcunMount/iam/api/grpc/iam/idp/v1"
	authclient "github.com/FangcunMount/iam/pkg/sdk/auth/client"
	"github.com/FangcunMount/iam/pkg/sdk/authz"
	"github.com/FangcunMount/iam/pkg/sdk/config"
	"github.com/FangcunMount/iam/pkg/sdk/identity"
	"github.com/FangcunMount/iam/pkg/sdk/idp"
	internaltransport "github.com/FangcunMount/iam/pkg/sdk/internal/transport"
	"google.golang.org/grpc"
)

// Client IAM 统一客户端。
type Client struct {
	conn *grpc.ClientConn
	cfg  *Config

	authClient         *authclient.Client
	authzClient        *authz.Client
	identityClient     *identity.Client
	guardianshipClient *identity.GuardianshipClient
	idpClient          *idp.Client
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
	authService := authnv1.NewAuthServiceClient(c.conn)
	jwksService := authnv1.NewJWKSServiceClient(c.conn)
	c.authClient = authclient.NewClient(authService, jwksService)

	authorizationService := authzv1.NewAuthorizationServiceClient(c.conn)
	c.authzClient = authz.NewClient(authorizationService)

	readService := identityv1.NewIdentityReadClient(c.conn)
	lifecycleService := identityv1.NewIdentityLifecycleClient(c.conn)
	c.identityClient = identity.NewClient(readService, lifecycleService)

	queryService := identityv1.NewGuardianshipQueryClient(c.conn)
	commandService := identityv1.NewGuardianshipCommandClient(c.conn)
	c.guardianshipClient = identity.NewGuardianshipClient(queryService, commandService)

	idpService := idpv1.NewIDPServiceClient(c.conn)
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

func (c *Client) Guardianship() *identity.GuardianshipClient {
	return c.guardianshipClient
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
