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

type bearerMethodAuthenticator struct {
	tokenVerifier tokenapp.Verifier
}

func (a *bearerMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.tokenVerifier == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "token verifier is not initialized")
	}
	payload, ok := selected.Payload.(BearerPayload)
	if !ok {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "invalid bearer payload")
	}
	claims, err := a.tokenVerifier.VerifyAccessToken(ctx, payload.Token)
	if err != nil {
		logger.L(ctx).Warnw("令牌验证失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodBearerToken),
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
				"auth_method": string(MethodBearerToken),
			},
		},
		CredentialID: meta.FromUint64(0),
	}, nil
}
