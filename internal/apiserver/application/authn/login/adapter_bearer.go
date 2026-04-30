package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type bearerAdapter struct {
	tokenVerifier tokenapp.Verifier
}

func newBearerAdapter(tokenVerifier tokenapp.Verifier) *bearerAdapter {
	return &bearerAdapter{tokenVerifier: tokenVerifier}
}

func (*bearerAdapter) Kind() SignInKind {
	return SignInKind(AuthTypeJWTToken)
}

func (*bearerAdapter) AuthType() AuthType {
	return AuthTypeJWTToken
}

func (*bearerAdapter) TryLegacy(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if req.JWTToken == nil {
		return nil, false
	}
	return BearerPayload{
		methodPayloadCommon: common,
		Token:               *req.JWTToken,
	}, true
}

func (*bearerAdapter) BuildExplicit(req SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if req.JWTToken == nil || *req.JWTToken == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "bearer token is required for bearer token authentication")
	}
	return BearerPayload{
		methodPayloadCommon: common,
		Token:               *req.JWTToken,
	}, nil
}

func (a *bearerAdapter) Reauthenticate(ctx context.Context, payload MethodPayload) (authentication.AuthDecision, error) {
	if a == nil || a.tokenVerifier == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "token verifier is not initialized")
	}
	bearerPayload, ok := payload.(BearerPayload)
	if !ok {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "invalid bearer payload")
	}
	claims, err := a.tokenVerifier.VerifyAccessToken(ctx, bearerPayload.Token)
	if err != nil {
		logger.L(ctx).Warnw("令牌验证失败",
			"action", logger.ActionLogin,
			"scenario", string(AuthTypeJWTToken),
			"error", err.Error(),
		)
		return authentication.AuthDecision{
			OK:      false,
			ErrCode: authentication.ErrInvalidCredential,
		}, nil
	}
	return authentication.AuthDecision{
		OK: true,
		Principal: &authentication.Principal{
			UserID:    claims.UserID,
			AccountID: claims.AccountID,
			TenantID:  claims.TenantID,
			AMR:       []string{"jwt"},
			Claims: map[string]any{
				"auth_method": string(AuthTypeJWTToken),
			},
		},
		CredentialID: meta.FromUint64(0),
	}, nil
}
