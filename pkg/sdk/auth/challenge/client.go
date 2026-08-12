package challenge

import (
	"context"
	"fmt"
	"net/http"

	"github.com/FangcunMount/iam/v3/pkg/sdk/auth/internal/restclient"
)

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

func (c *Client) SendLoginPhoneOTP(ctx context.Context, req SendLoginPhoneOTPRequest) (*MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out MessageResponse
	if err := c.do(ctx, http.MethodPost, "/authn/challenges/phone-otp", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) StartWechatOpenAuthorize(ctx context.Context, req WechatOpenAuthorizeRequest) (*WechatOpenAuthorizeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out WechatOpenAuthorizeResponse
	if err := c.do(ctx, http.MethodPost, "/authn/wechat-open/authorize", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, in any, out any) error {
	if c == nil || c.rest == nil {
		return fmt.Errorf("challenge: client is nil")
	}
	return c.rest.DoJSON(ctx, method, path, in, out)
}
