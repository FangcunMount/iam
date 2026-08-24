package externalidentity

import (
	"context"
	"errors"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// AuthenticationStageError preserves the former SignIn stage for provider exchange failures.
type AuthenticationStageError struct {
	cause error
}

func (e *AuthenticationStageError) Error() string { return e.cause.Error() }
func (e *AuthenticationStageError) Unwrap() error { return e.cause }
func (e *AuthenticationStageError) Cause() error  { return e.cause }

func AuthenticationCause(err error) (error, bool) {
	var stageError *AuthenticationStageError
	if !errors.As(err, &stageError) {
		return nil, false
	}
	return stageError.Cause(), true
}

// MapLoginProofError maps provider-neutral IDP failures back to the existing SignIn contract.
func MapLoginProofError(ctx context.Context, err error, credentialKind string) error {
	resolutionError, ok := idpresolver.AsResolutionError(err)
	if !ok {
		return err
	}
	logResolutionError(ctx, resolutionError, credentialKind)
	provider := providerName(resolutionError.Provider)
	cause := rootCause(resolutionError)

	switch resolutionError.Kind {
	case idpresolver.ErrorInvalidRequest:
		return perrors.WithCode(code.ErrInvalidArgument, "%s external identity proof is incomplete", provider)
	case idpresolver.ErrorUnavailable:
		return perrors.WithCode(code.ErrProofBuildFailed, "%s app configuration service not available", provider)
	case idpresolver.ErrorProviderConfig:
		return perrors.WithCode(code.ErrProofBuildFailed, "wecom agent_id is required in server configuration")
	case idpresolver.ErrorAppQueryFailed:
		return perrors.WithCode(code.ErrProofBuildFailed, "failed to query %s app: %v", provider, cause)
	case idpresolver.ErrorAppNotFound:
		return perrors.WithCode(code.ErrProofBuildFailed, "%s app not found: %s", provider, resolutionError.Realm)
	case idpresolver.ErrorAppDisabled:
		return perrors.WithCode(code.ErrProofBuildFailed, "%s app is disabled: %s", provider, resolutionError.Realm)
	case idpresolver.ErrorCredentialMissing:
		return perrors.WithCode(code.ErrProofBuildFailed, "%s app credentials not found", provider)
	case idpresolver.ErrorSecretDecryptFailed:
		if resolutionError.Provider == idpidentity.ProviderWecom {
			return perrors.WithCode(code.ErrProofBuildFailed, "failed to decrypt wecom corp secret: %v", cause)
		}
		return perrors.WithCode(code.ErrProofBuildFailed, "failed to decrypt app secret: %v", cause)
	case idpresolver.ErrorAppTypeMismatch:
		return perrors.WithCode(code.ErrWechatAppTypeMismatch,
			"wechat app type mismatch: expected %s, got %s", resolutionError.Expected, resolutionError.Actual)
	case idpresolver.ErrorProviderExchange, idpresolver.ErrorInvalidProviderReply:
		return &AuthenticationStageError{cause: fmt.Errorf("%s: %w", exchangeFailurePrefix(resolutionError.Provider), cause)}
	default:
		return err
	}
}

// MapLinkingError maps IDP failures to the existing linking error surface.
func MapLinkingError(err error) error {
	resolutionError, ok := idpresolver.AsResolutionError(err)
	if !ok {
		return err
	}
	provider := providerName(resolutionError.Provider)
	cause := rootCause(resolutionError)

	switch resolutionError.Kind {
	case idpresolver.ErrorInvalidRequest:
		return perrors.WithCode(code.ErrInvalidArgument, "%s external identity proof is incomplete", provider)
	case idpresolver.ErrorUnavailable:
		return perrors.WithCode(code.ErrInvalidArgument, "%s app configuration service is not available", provider)
	case idpresolver.ErrorProviderConfig:
		return perrors.WithCode(code.ErrInvalidArgument, "corp_id, code and wecom agent_id are required")
	case idpresolver.ErrorAppQueryFailed:
		return perrors.WithCode(code.ErrInvalidArgument, "failed to query %s app: %v", provider, cause)
	case idpresolver.ErrorAppNotFound, idpresolver.ErrorAppDisabled, idpresolver.ErrorCredentialMissing:
		return perrors.WithCode(code.ErrInvalidArgument, "%s app is not available", provider)
	case idpresolver.ErrorSecretDecryptFailed:
		return perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt %s app secret: %v", provider, cause)
	case idpresolver.ErrorAppTypeMismatch:
		return perrors.WithCode(code.ErrWechatAppTypeMismatch,
			"wechat app type mismatch: expected %s, got %s", resolutionError.Expected, resolutionError.Actual)
	case idpresolver.ErrorProviderExchange, idpresolver.ErrorInvalidProviderReply:
		return perrors.WithCode(code.ErrInvalidCredential, "failed to exchange %s code: %v", provider, cause)
	default:
		return err
	}
}

// MapSignupError maps IDP failures to the existing mini-program signup surface.
func MapSignupError(err error) error {
	resolutionError, ok := idpresolver.AsResolutionError(err)
	if !ok {
		return err
	}
	cause := rootCause(resolutionError)

	switch resolutionError.Kind {
	case idpresolver.ErrorInvalidRequest:
		return perrors.WithCode(code.ErrInvalidArgument, "appid and jscode are required for wechat mini program")
	case idpresolver.ErrorUnavailable:
		return perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	case idpresolver.ErrorAppQueryFailed:
		return perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", cause)
	case idpresolver.ErrorAppNotFound:
		return perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", resolutionError.Realm)
	case idpresolver.ErrorAppDisabled:
		return perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", resolutionError.Realm)
	case idpresolver.ErrorCredentialMissing:
		return perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	case idpresolver.ErrorSecretDecryptFailed:
		return perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt wechat app secret: %v", cause)
	case idpresolver.ErrorAppTypeMismatch:
		return perrors.WithCode(code.ErrWechatAppTypeMismatch,
			"wechat app type mismatch: expected %s, got %s", resolutionError.Expected, resolutionError.Actual)
	case idpresolver.ErrorProviderExchange, idpresolver.ErrorInvalidProviderReply:
		return perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", cause)
	default:
		return err
	}
}

func logResolutionError(ctx context.Context, err *idpresolver.ResolutionError, credentialKind string) {
	logger.L(ctx).Errorw("外部身份解析失败",
		"action", logger.ActionLogin,
		"credential_kind", credentialKind,
		"provider", err.Provider,
		"realm", err.Realm,
		"error_kind", err.Kind,
	)
}

func providerName(provider idpidentity.Provider) string {
	if provider == idpidentity.ProviderWecom {
		return "wecom"
	}
	return "wechat"
}

func exchangeFailurePrefix(provider idpidentity.Provider) string {
	switch provider {
	case idpidentity.ProviderWechatMinip:
		return "failed to exchange wx minip code"
	case idpidentity.ProviderWechatOpen:
		return "failed to exchange wx open code"
	case idpidentity.ProviderWecom:
		return "failed to exchange wecom code"
	default:
		return "failed to exchange external identity code"
	}
}

func rootCause(err *idpresolver.ResolutionError) error {
	if cause := errors.Unwrap(err); cause != nil {
		return cause
	}
	return err
}
