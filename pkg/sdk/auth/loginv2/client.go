package loginv2

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

	sdkerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

// Client calls the REST AuthN v2 login endpoint.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	headers    http.Header
}

// Option customizes the REST AuthN v2 login client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithHeader adds a static header to every request.
func WithHeader(key, value string) Option {
	return func(c *Client) {
		if strings.TrimSpace(key) == "" {
			return
		}
		c.headers.Add(key, value)
	}
}

// NewClient creates a REST AuthN v2 login client.
//
// baseURL may be the IAM origin or an IAM /api/v2 URL. The client normalizes it
// to call POST /api/v2/authn/login.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("loginv2: base URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("loginv2: parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("loginv2: base URL must be absolute")
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

// Login posts an explicit REST AuthN v2 login request.
func (c *Client) Login(ctx context.Context, req LoginRequest) (*TokenPair, error) {
	if c == nil {
		return nil, fmt.Errorf("loginv2: client is nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("loginv2: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.loginURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("loginv2: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("loginv2: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("loginv2: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errorFromEnvelope(resp.StatusCode, respBody)
	}
	return decodeTokenPair(resp.StatusCode, respBody)
}

func (c *Client) loginURL() string {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/authn/login"
	return u.String()
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

func decodeTokenPair(statusCode int, body []byte) (*TokenPair, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && (envelope.Code != nil || envelope.Message != "" || len(envelope.Data) > 0) {
		if envelope.Code != nil && *envelope.Code != 0 {
			return nil, iamError(statusCode, *envelope.Code, envelope.Message, nil)
		}
		if len(envelope.Data) == 0 {
			return nil, fmt.Errorf("loginv2: success response missing data")
		}
		var tokenPair TokenPair
		if err := json.Unmarshal(envelope.Data, &tokenPair); err != nil {
			return nil, fmt.Errorf("loginv2: decode response data: %w", err)
		}
		return &tokenPair, nil
	}

	var tokenPair TokenPair
	if err := json.Unmarshal(body, &tokenPair); err != nil {
		return nil, fmt.Errorf("loginv2: decode response: %w", err)
	}
	return &tokenPair, nil
}

func errorFromEnvelope(statusCode int, body []byte) error {
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
