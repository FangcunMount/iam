package loginidentity

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

func WithBearerToken(token string) Option {
	return func(opts *clientOptions) {
		opts.restOptions = append(opts.restOptions, restclient.WithBearerToken(token))
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

func (c *Client) List(ctx context.Context) (*ListResponse, error) {
	var out ListResponse
	if err := c.do(ctx, http.MethodGet, "/authn/login-identities", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SendPhoneLinkChallenge(ctx context.Context, req LinkPhoneChallengeRequest) (*MessageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out MessageResponse
	if err := c.do(ctx, http.MethodPost, "/authn/login-identities/phone/challenge", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) LinkPhone(ctx context.Context, req LinkPhoneRequest) (*LinkResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out LinkResponse
	if err := c.do(ctx, http.MethodPost, "/authn/login-identities/phone", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) LinkWechatMiniProgram(ctx context.Context, req LinkWechatMiniProgramRequest) (*LinkResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out LinkResponse
	if err := c.do(ctx, http.MethodPost, "/authn/login-identities/wechat-miniprogram", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) LinkWecom(ctx context.Context, req LinkWecomRequest) (*LinkResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out LinkResponse
	if err := c.do(ctx, http.MethodPost, "/authn/login-identities/wecom", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Unlink(ctx context.Context, id string) (*MessageResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, restclient.InvalidArgument("login identity id is required")
	}
	var out MessageResponse
	path := "/authn/login-identities/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodDelete, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, in any, out any) error {
	if c == nil || c.rest == nil {
		return fmt.Errorf("loginidentity: client is nil")
	}
	return c.rest.DoJSON(ctx, method, path, in, out)
}
