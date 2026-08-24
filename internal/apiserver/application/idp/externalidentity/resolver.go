package externalidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	wechatapp "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
)

// Resolver resolves a one-time provider proof into a request-scoped identity.
type Resolver interface {
	Resolve(context.Context, ResolveRequest) (domain.ExternalIdentity, error)
}

type ResolveRequest struct {
	Provider domain.Provider
	Realm    string
	Code     string
}

// Config contains provider settings owned by IDP rather than AuthN.
type Config struct {
	WeComAgentID string
}

// ProviderExchanger is the provider SDK anti-corruption port owned by IDP.
type ProviderExchanger interface {
	ExchangeWxMinipCode(ctx context.Context, appID, appSecret, jsCode string) (openID, unionID string, err error)
	ExchangeWxOpenCode(ctx context.Context, appID, appSecret, code string) (openID, unionID string, err error)
	ExchangeWecomCode(ctx context.Context, corpID, agentID, corpSecret, code string) (openUserID, userID string, err error)
}

type Dependencies struct {
	Apps      wechatapp.Repository
	Vault     wechatapp.SecretVault
	Exchanger ProviderExchanger
	Config    Config
	Now       func() time.Time
}

type resolver struct {
	deps Dependencies
}

func NewResolver(deps Dependencies) Resolver {
	return &resolver{deps: deps}
}

// ErrorKind classifies resolution failures without coupling IDP to AuthN codes.
type ErrorKind string

const (
	ErrorInvalidRequest       ErrorKind = "invalid_request"
	ErrorUnavailable          ErrorKind = "unavailable"
	ErrorProviderConfig       ErrorKind = "provider_configuration_missing"
	ErrorAppQueryFailed       ErrorKind = "app_query_failed"
	ErrorAppNotFound          ErrorKind = "app_not_found"
	ErrorAppDisabled          ErrorKind = "app_disabled"
	ErrorAppTypeMismatch      ErrorKind = "app_type_mismatch"
	ErrorCredentialMissing    ErrorKind = "credential_missing"
	ErrorSecretDecryptFailed  ErrorKind = "secret_decrypt_failed"
	ErrorProviderExchange     ErrorKind = "provider_exchange_failed"
	ErrorInvalidProviderReply ErrorKind = "invalid_provider_response"
)

// ResolutionError carries safe classification and an internal root cause.
type ResolutionError struct {
	Kind     ErrorKind
	Provider domain.Provider
	Realm    string
	Expected wechatapp.AppType
	Actual   wechatapp.AppType
	cause    error
}

func (e *ResolutionError) Error() string {
	return fmt.Sprintf("external identity resolution failed: provider=%s realm=%s kind=%s", e.Provider, e.Realm, e.Kind)
}

func (e *ResolutionError) Unwrap() error { return e.cause }

func KindOf(err error) (ErrorKind, bool) {
	var resolutionError *ResolutionError
	if !errors.As(err, &resolutionError) {
		return "", false
	}
	return resolutionError.Kind, true
}

func AsResolutionError(err error) (*ResolutionError, bool) {
	var resolutionError *ResolutionError
	if !errors.As(err, &resolutionError) {
		return nil, false
	}
	return resolutionError, true
}

func (r *resolver) Resolve(ctx context.Context, request ResolveRequest) (domain.ExternalIdentity, error) {
	request.Realm = strings.TrimSpace(request.Realm)
	request.Code = strings.TrimSpace(request.Code)
	if !request.Provider.Validate() || request.Realm == "" || request.Code == "" {
		return domain.ExternalIdentity{}, r.failure(request, ErrorInvalidRequest, nil)
	}
	if r == nil || r.deps.Apps == nil || r.deps.Vault == nil || r.deps.Exchanger == nil {
		return domain.ExternalIdentity{}, r.failure(request, ErrorUnavailable, nil)
	}

	if request.Provider == domain.ProviderWecom && strings.TrimSpace(r.deps.Config.WeComAgentID) == "" {
		return domain.ExternalIdentity{}, r.failure(request, ErrorProviderConfig, nil)
	}

	secret, err := r.resolveSecret(ctx, request)
	if err != nil {
		return domain.ExternalIdentity{}, err
	}

	identifiers, err := r.exchange(ctx, request, secret)
	if err != nil {
		return domain.ExternalIdentity{}, err
	}
	identity, err := domain.New(request.Provider, request.Realm, identifiers, r.now())
	if err != nil {
		return domain.ExternalIdentity{}, r.failure(request, ErrorInvalidProviderReply, err)
	}
	return identity, nil
}

func (r *resolver) resolveSecret(ctx context.Context, request ResolveRequest) (string, error) {
	app, err := r.deps.Apps.GetByAppID(ctx, request.Realm)
	if err != nil {
		return "", r.failure(request, ErrorAppQueryFailed, err)
	}
	if app == nil {
		return "", r.failure(request, ErrorAppNotFound, nil)
	}
	if expected := expectedAppType(request.Provider); expected != "" && app.Type != expected {
		failure := r.failure(request, ErrorAppTypeMismatch, nil)
		failure.Expected = expected
		failure.Actual = app.Type
		return "", failure
	}
	if !app.IsEnabled() {
		return "", r.failure(request, ErrorAppDisabled, nil)
	}
	if app.Cred == nil || app.Cred.Auth == nil {
		return "", r.failure(request, ErrorCredentialMissing, nil)
	}
	plain, err := r.deps.Vault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", r.failure(request, ErrorSecretDecryptFailed, err)
	}
	return string(plain), nil
}

func (r *resolver) exchange(ctx context.Context, request ResolveRequest, secret string) ([]domain.Identifier, error) {
	switch request.Provider {
	case domain.ProviderWechatMinip:
		openID, unionID, err := r.deps.Exchanger.ExchangeWxMinipCode(ctx, request.Realm, secret, request.Code)
		if err != nil {
			return nil, r.failure(request, ErrorProviderExchange, err)
		}
		return identifiers(
			identifier(domain.IdentifierOpenID, openID),
			identifier(domain.IdentifierUnionID, unionID),
		), nil
	case domain.ProviderWechatOpen:
		openID, unionID, err := r.deps.Exchanger.ExchangeWxOpenCode(ctx, request.Realm, secret, request.Code)
		if err != nil {
			return nil, r.failure(request, ErrorProviderExchange, err)
		}
		return identifiers(
			identifier(domain.IdentifierOpenID, openID),
			identifier(domain.IdentifierUnionID, unionID),
		), nil
	case domain.ProviderWecom:
		openUserID, userID, err := r.deps.Exchanger.ExchangeWecomCode(ctx, request.Realm, strings.TrimSpace(r.deps.Config.WeComAgentID), secret, request.Code)
		if err != nil {
			return nil, r.failure(request, ErrorProviderExchange, err)
		}
		return identifiers(
			identifier(domain.IdentifierUserID, userID),
			identifier(domain.IdentifierOpenUserID, openUserID),
		), nil
	default:
		return nil, r.failure(request, ErrorInvalidRequest, nil)
	}
}

func identifier(kind domain.IdentifierKind, value string) *domain.Identifier {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	identifier, err := domain.NewIdentifier(kind, value)
	if err != nil {
		return nil
	}
	return &identifier
}

func identifiers(values ...*domain.Identifier) []domain.Identifier {
	result := make([]domain.Identifier, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, *value)
		}
	}
	return result
}

func expectedAppType(provider domain.Provider) wechatapp.AppType {
	switch provider {
	case domain.ProviderWechatMinip:
		return wechatapp.MiniProgram
	case domain.ProviderWechatOpen:
		return wechatapp.OpenPlatformWebsite
	case domain.ProviderWecom:
		// WeCom applications use the existing MP registry type. Keep the mapping
		// explicit so a CorpID cannot resolve credentials from another app type.
		return wechatapp.MP
	default:
		return ""
	}
}

func (r *resolver) now() time.Time {
	if r.deps.Now != nil {
		return r.deps.Now()
	}
	return time.Now()
}

func (r *resolver) failure(request ResolveRequest, kind ErrorKind, cause error) *ResolutionError {
	return &ResolutionError{
		Kind:     kind,
		Provider: request.Provider,
		Realm:    request.Realm,
		cause:    cause,
	}
}
