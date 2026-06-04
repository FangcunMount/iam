package request

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type openAPISpec struct {
	Paths      map[string]map[string]openAPIOperation `yaml:"paths"`
	Components struct {
		Schemas map[string]openAPISchema `yaml:"schemas"`
	} `yaml:"components"`
}

type openAPIOperation struct {
	Description string `yaml:"description"`
	RequestBody struct {
		Content map[string]struct {
			Schema   openAPISchema `yaml:"schema"`
			Examples map[string]struct {
				Value map[string]any `yaml:"value"`
			} `yaml:"examples"`
		} `yaml:"content"`
	} `yaml:"requestBody"`
	Responses map[string]struct {
		Content map[string]struct {
			Schema openAPISchema `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"responses"`
}

type openAPISchema struct {
	Ref         string                   `yaml:"$ref"`
	Type        string                   `yaml:"type"`
	Description string                   `yaml:"description"`
	Enum        []string                 `yaml:"enum"`
	Properties  map[string]openAPISchema `yaml:"properties"`
}

func TestLoginV2OpenAPIContractMatchesRequestValidation(t *testing.T) {
	spec := loadOpenAPISpec(t, "api/rest/authn.v2.yaml")

	loginSchema := spec.schema(t, "LoginV2Request")
	require.ElementsMatch(t, []string{"password", "phone_otp", "wechat", "wechat_scan", "wecom"}, loginSchema.Properties["auth_method"].Enum)
	require.Contains(t, loginSchema.Properties["method_payload"].Description, "wechat_scan")

	for _, method := range loginSchema.Properties["auth_method"].Enum {
		req := LoginV2Request{
			AuthMethod:    method,
			MethodPayload: json.RawMessage(`{}`),
		}
		require.NoError(t, req.Validate(), "OpenAPI auth_method %q must be accepted by request validation", method)
	}

	req := LoginV2Request{
		AuthMethod:    "jwt_token",
		MethodPayload: json.RawMessage(`{"access_token":"token"}`),
	}
	require.Error(t, req.Validate(), "jwt_token must stay out of the public REST v2 login contract")

	examples := spec.Paths["/authn/login"]["post"].RequestBody.Content["application/json"].Examples
	require.Contains(t, examples, "wechat_scan")
	require.Equal(t, "wechat_scan", examples["wechat_scan"].Value["auth_method"])
	require.Contains(t, examples, "wecom")
	require.Equal(t, "wecom", examples["wecom"].Value["auth_method"])

	okSchema := spec.Paths["/authn/login"]["post"].Responses["200"].Content["application/json"].Schema
	require.Contains(t, okSchema.Ref, "TokenPair")

	loginAuthorizeSchema := spec.Paths["/authn/wechat-open/authorize"]["post"].RequestBody.Content["application/json"].Schema
	require.Contains(t, loginAuthorizeSchema.Ref, "WechatOpenLoginAuthorizeRequest")

	linkAuthorizeSchema := spec.Paths["/authn/login-identities/wechat-open/authorize"]["post"].RequestBody.Content["application/json"].Schema
	require.Contains(t, linkAuthorizeSchema.Ref, "LinkWechatOpenAuthorizeRequest")
}

func (s openAPISpec) schema(t *testing.T, name string) openAPISchema {
	t.Helper()
	if schema, ok := s.Components.Schemas[name]; ok {
		return schema
	}
	for key, schema := range s.Components.Schemas {
		if strings.HasSuffix(key, "."+name) {
			return schema
		}
	}
	t.Fatalf("schema %s not found", name)
	return openAPISchema{}
}

func loadOpenAPISpec(t *testing.T, rel string) openAPISpec {
	t.Helper()

	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, yaml.Unmarshal(data, &spec))
	return spec
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found")
		dir = parent
	}
}
