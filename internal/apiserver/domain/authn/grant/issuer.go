package grant

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Dependencies 是认证结果颁发所需的领域协作者。
type Dependencies struct {
	AdmissionPolicy   admissiondomain.Policy
	SessionCreator    SessionCreator
	TokenSetMinter    TokenSetMinter
	RefreshTokenSaver RefreshTokenSaver
}

// issuer 是认证结果颁发器的实现。
type issuer struct {
	admissionPolicy   admissiondomain.Policy
	sessionCreator    SessionCreator
	tokenSetMinter    TokenSetMinter
	refreshTokenSaver RefreshTokenSaver
}

// NewIssuer 创建认证结果颁发器。
func NewIssuer(deps Dependencies) Issuer {
	return &issuer{
		admissionPolicy:   deps.AdmissionPolicy,
		sessionCreator:    deps.SessionCreator,
		tokenSetMinter:    deps.TokenSetMinter,
		refreshTokenSaver: deps.RefreshTokenSaver,
	}
}

// Issue 在准入通过后建立 Session、颁发 TokenSet，并保存初始 RefreshToken。
func (s *issuer) Issue(ctx context.Context, principal *authentication.Principal) (*AuthenticationGrant, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	if err := admissiondomain.Require(ctx, s.admissionPolicy, admissiondomain.Subject{
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
	}); err != nil {
		return nil, err
	}
	if s.sessionCreator == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "session creator is not configured")
	}

	sess, err := s.sessionCreator.Create(ctx, principal)
	if err != nil {
		if perrors.IsCode(err, code.ErrInvalidArgument) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}
	if s.tokenSetMinter == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token set minter is not configured")
	}

	set, err := s.tokenSetMinter.MintTokenSet(ctx, principal, sess)
	if err != nil {
		return nil, err
	}
	if s.refreshTokenSaver == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "refresh token saver is not configured")
	}
	if err := s.refreshTokenSaver.SaveRefreshToken(ctx, set.RefreshToken); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	return NewAuthenticationGrant(sess, set), nil
}
