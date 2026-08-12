package restclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	headers    http.Header
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithHeader(key, value string) Option {
	return func(c *Client) {
		if strings.TrimSpace(key) == "" {
			return
		}
		c.headers.Add(key, value)
	}
}

func WithBearerToken(token string) Option {
	return WithHeader("Authorization", "Bearer "+strings.TrimSpace(token))
}

func New(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("restclient: base URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("restclient: parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("restclient: base URL must be absolute")
	}
	parsed.Path = withAPIV2Path(parsed.Path)
	parsed.RawQuery = ""
	parsed.Fragment = ""

	client := &Client{
		baseURL:    parsed,
		httpClient: http.DefaultClient,
		headers:    make(http.Header),
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

func (c *Client) DoJSON(ctx context.Context, method, path string, in any, out any) error {
	if c == nil {
		return fmt.Errorf("restclient: client is nil")
	}

	body, err := encodeBody(in)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, c.url(path), body)
	if err != nil {
		return fmt.Errorf("restclient: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	if in != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for key, values := range c.headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("restclient: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("restclient: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ErrorFromEnvelope(resp.StatusCode, respBody)
	}
	if out == nil {
		return nil
	}
	return DecodeEnvelope(resp.StatusCode, respBody, out)
}

func (c *Client) url(path string) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return u.String()
}

func encodeBody(in any) (io.Reader, error) {
	if in == nil {
		return http.NoBody, nil
	}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("restclient: encode request: %w", err)
	}
	return bytes.NewReader(body), nil
}

func withAPIV2Path(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/api/v2"
	}
	if strings.HasSuffix(path, "/api/v2") {
		return path
	}
	return path + "/api/v2"
}

type responseEnvelope struct {
	Code      *int            `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	Reference string          `json:"reference,omitempty"`
}

func DecodeEnvelope(statusCode int, body []byte, out any) error {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && (envelope.Code != nil || envelope.Message != "" || len(envelope.Data) > 0) {
		if envelope.Code != nil && *envelope.Code != 0 {
			return iamError(statusCode, *envelope.Code, envelope.Message, nil)
		}
		if len(envelope.Data) == 0 {
			return fmt.Errorf("restclient: success response missing data")
		}
		if string(envelope.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("restclient: decode response data: %w", err)
		}
		return nil
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("restclient: decode response: %w", err)
	}
	return nil
}

func ErrorFromEnvelope(statusCode int, body []byte) error {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil {
		code := statusCode
		if envelope.Code != nil {
			code = *envelope.Code
		}
		return iamError(statusCode, code, envelope.Message, nil)
	}
	return iamError(statusCode, statusCode, strings.TrimSpace(string(body)), nil)
}

func InvalidArgument(message string) error {
	return &sdkerrors.IAMError{
		Code:     codes.InvalidArgument.String(),
		Message:  message,
		GRPCCode: codes.InvalidArgument,
	}
}

func iamError(statusCode int, code int, message string, cause error) error {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(statusCode)
	}
	return &sdkerrors.IAMError{
		Code:     strconv.Itoa(code),
		Message:  message,
		GRPCCode: grpcCodeFromHTTPStatus(statusCode),
		Cause:    cause,
	}
}

func grpcCodeFromHTTPStatus(statusCode int) codes.Code {
	switch statusCode {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case 499:
		return codes.Canceled
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	default:
		if statusCode >= http.StatusInternalServerError {
			return codes.Internal
		}
		return codes.Unknown
	}
}
