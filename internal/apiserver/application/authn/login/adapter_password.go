package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type passwordAdapter struct{}

func newPasswordAdapter() passwordAdapter {
	return passwordAdapter{}
}

func (passwordAdapter) Kind() SignInKind {
	return SignInKind(credDomain.CredPassword)
}

func (passwordAdapter) AuthType() AuthType {
	return AuthTypePassword
}

func (passwordAdapter) TryLegacy(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if req.Username == nil || req.Password == nil {
		return nil, false
	}
	return PasswordPayload{
		methodPayloadCommon: common,
		Username:            *req.Username,
		Password:            *req.Password,
	}, true
}

func (passwordAdapter) BuildExplicit(req SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if req.Username == nil || *req.Username == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "username is required for password authentication")
	}
	if req.Password == nil || *req.Password == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required for password authentication")
	}
	return PasswordPayload{
		methodPayloadCommon: common,
		Username:            *req.Username,
		Password:            *req.Password,
	}, nil
}

func (passwordAdapter) PrepareProof(_ context.Context, payload MethodPayload) (authentication.AuthCredential, error) {
	passwordPayload, ok := payload.(PasswordPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid password payload")
	}
	return authentication.NewPasswordCredential(authentication.PasswordProofSpec{
		TenantID:  passwordPayload.TenantID,
		RemoteIP:  passwordPayload.RemoteIP,
		UserAgent: passwordPayload.UserAgent,
		Username:  passwordPayload.Username,
		Password:  passwordPayload.Password,
	})
}
