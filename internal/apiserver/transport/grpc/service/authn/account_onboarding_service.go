package authn

import (
	"context"
	"strings"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	onboardingApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	accountDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *accountOnboardingServer) CreateOperationAccount(ctx context.Context, req *authnv1.CreateOperationAccountRequest) (*authnv1.CreateOperationAccountResponse, error) {
	if s.accountOnboarder == nil {
		return nil, status.Error(codes.Unimplemented, "account onboarding service not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	scopedTenantID, err := meta.ParseID(strings.TrimSpace(req.GetScopedTenantId()))
	if err != nil || scopedTenantID.IsZero() {
		return nil, status.Error(codes.InvalidArgument, "scoped_tenant_id is required")
	}

	password := strings.TrimSpace(req.GetPassword())
	if password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	name := strings.TrimSpace(req.GetName())
	existingUserIDText := strings.TrimSpace(req.GetExistingUserId())
	if existingUserIDText == "" && name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required when existing_user_id is empty")
	}

	existingUserID, err := parseOptionalMetaID(existingUserIDText)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid existing_user_id")
	}

	var phone meta.Phone
	phoneText := strings.TrimSpace(req.GetPhone())
	if phoneText != "" {
		phone, err = meta.NewPhone(phoneText)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid phone")
		}
	}

	var email meta.Email
	emailText := strings.TrimSpace(req.GetEmail())
	if emailText != "" {
		email, err = meta.NewEmail(emailText)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid email")
		}
	}

	result, err := s.accountOnboarder.Onboard(ctx, onboardingApp.OnboardingRequest{
		Name:           name,
		Phone:          phone,
		Email:          email,
		ExistingUserID: existingUserID,
		OperaLoginID:   strings.TrimSpace(req.GetOperaLoginId()),
		ScopedTenantID: scopedTenantID,
		AccountType:    accountDomain.TypeOpera,
		CredentialType: onboardingApp.CredTypePassword,
		Password:       &password,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authnv1.CreateOperationAccountResponse{
		UserId:       result.UserID.String(),
		AccountId:    result.AccountID.String(),
		CredentialId: result.CredentialID.String(),
		ExternalId:   string(result.ExternalID),
		IsNewUser:    result.IsNewUser,
		IsNewAccount: result.IsNewAccount,
	}, nil
}
