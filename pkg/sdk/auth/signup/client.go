package signup

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/FangcunMount/iam/v2/pkg/sdk/auth/internal/restclient"
)

const SeedMockSecretHeader = "X-IAM-Seed-Secret"

type Client struct {
	rest *restclient.Client
}

type Option func(*clientOptions)

type clientOptions struct {
	restOptions []restclient.Option
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(opts *clientOptions) {
		opts.restOptions = append(opts.restOptions, restclient.WithHTTPClient(httpClient))
	}
}

func WithHeader(key, value string) Option {
	return func(opts *clientOptions) {
		opts.restOptions = append(opts.restOptions, restclient.WithHeader(key, value))
	}
}

func WithSeedMockSecret(secret string) Option {
	return func(opts *clientOptions) {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			return
		}
		opts.restOptions = append(opts.restOptions, restclient.WithHeader(SeedMockSecretHeader, secret))
	}
}

func NewClient(baseURL string, opts ...Option) (*Client, error) {
	options := clientOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	rest, err := restclient.New(baseURL, options.restOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest}, nil
}

func (c *Client) SignUpWithWechatMiniProgram(ctx context.Context, req WechatMiniProgramRequest) (*SignupResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out SignupResult
	if err := c.do(ctx, http.MethodPost, "/authn/signups/wechat-miniprogram", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) EnsureMockConsumer(ctx context.Context, req EnsureMockConsumerRequest) (*EnsureMockConsumerResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out EnsureMockConsumerResult
	if err := c.do(ctx, http.MethodPost, "/internal/authn/mock-consumers/ensure", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, in any, out any) error {
	if c == nil || c.rest == nil {
		return fmt.Errorf("signup: client is nil")
	}
	return c.rest.DoJSON(ctx, method, path, in, out)
}
