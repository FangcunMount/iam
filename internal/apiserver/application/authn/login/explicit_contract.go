package login

import (
	"encoding/json"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// PublicAuthTypes returns the auth_method values exposed by the v2 public
// login contract. The set is derived from the application sign-in catalog so
// transports do not maintain their own method list.
func PublicAuthTypes() []AuthType {
	catalog := newDefaultSignInAdapterCatalog(signInAdapterDeps{})
	types := make([]AuthType, 0, len(catalog.ordered))
	for _, adapter := range catalog.adapters() {
		authType := adapter.AuthType()
		if authType == AuthTypeJWTToken {
			continue
		}
		types = append(types, authType)
	}
	return types
}

// IsPublicAuthType reports whether method is accepted by the v2 public login
// contract.
func IsPublicAuthType(method string) bool {
	authType := AuthType(strings.TrimSpace(method))
	for _, allowed := range PublicAuthTypes() {
		if authType == allowed {
			return true
		}
	}
	return false
}

// BuildExplicitLoginRequest converts the v2 public auth_method +
// method_payload wire contract into the application LoginRequest.
func BuildExplicitLoginRequest(method string, payload json.RawMessage) (LoginRequest, error) {
	authType := AuthType(strings.TrimSpace(method))
	if !IsPublicAuthType(string(authType)) {
		return LoginRequest{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", method)
	}
	if len(payload) == 0 {
		return LoginRequest{}, perrors.WithCode(code.ErrInvalidArgument, "method_payload is required")
	}

	adapter := explicitLoginPayloadAdapters()[authType]
	if adapter == nil {
		return LoginRequest{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", method)
	}
	return adapter.Build(payload)
}
