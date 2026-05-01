package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// CredentialBinder 负责开通流程中的凭据绑定、持久化和幂等复用。
type CredentialBinder struct {
	hasher authentication.PasswordHasher
}

func newCredentialBinder(hasher authentication.PasswordHasher) *CredentialBinder {
	return &CredentialBinder{hasher: hasher}
}

func (i *CredentialBinder) Bind(
	ctx context.Context,
	repo credDomain.Repository,
	accountID meta.ID,
	creationParams *domain.CreationParams,
	req OnboardingRequest,
) (*credDomain.Credential, error) {
	issuer := credDomain.NewIssuer(i.hasher)
	credential, err := i.buildCredential(ctx, issuer, accountID, creationParams, req)
	if err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, credential); err != nil {
		if perrors.IsCode(err, code.ErrCredentialExists) {
			credType := mapCredentialType(req.CredentialType)
			existing, getErr := repo.GetByAccountIDAndType(ctx, accountID, credType)
			if getErr != nil {
				return nil, perrors.WithCode(code.ErrDatabase, "failed to reuse credential: %v", getErr)
			}
			return existing, nil
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
	}

	return credential, nil
}

func (i *CredentialBinder) buildCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	creationParams *domain.CreationParams,
	req OnboardingRequest,
) (*credDomain.Credential, error) {
	switch req.CredentialType {
	case CredTypePassword:
		if req.Password == nil || *req.Password == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
		}
		return issuer.IssuePassword(ctx, credDomain.IssuePasswordRequest{
			AccountID:     accountID,
			PlainPassword: *req.Password,
		})

	case CredTypePhone:
		return issuer.IssuePhoneOTP(ctx, credDomain.IssuePhoneOTPRequest{
			AccountID: accountID,
			Phone:     req.Phone,
		})

	case CredTypeWechat:
		if creationParams == nil || creationParams.OpenID == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "openid is required for wechat credential")
		}
		idpIdentifier := creationParams.OpenID
		if creationParams.UnionID != "" {
			idpIdentifier = creationParams.UnionID
		}
		appID := ""
		if req.WechatAppID != nil {
			appID = *req.WechatAppID
		}
		return issuer.IssueWechatMinip(ctx, credDomain.IssueOAuthRequest{
			AccountID:     accountID,
			IDPIdentifier: idpIdentifier,
			AppID:         appID,
			ParamsJSON:    req.ParamsJSON,
		})

	case CredTypeWecom:
		if req.WecomUserID == nil || *req.WecomUserID == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom userid is required")
		}
		appID := ""
		if req.WecomCorpID != nil {
			appID = *req.WecomCorpID
		}
		return issuer.IssueWecom(ctx, credDomain.IssueOAuthRequest{
			AccountID:     accountID,
			IDPIdentifier: *req.WecomUserID,
			AppID:         appID,
			ParamsJSON:    req.ParamsJSON,
		})

	default:
		return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported credential type: %s", req.CredentialType)
	}
}
