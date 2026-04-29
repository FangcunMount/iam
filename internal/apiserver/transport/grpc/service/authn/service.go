package authn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	onboardingApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	tokenApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	accountDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Service 聚合 authn 模块的 gRPC 服务
type Service struct {
	auth       authServiceServer
	onboarding accountOnboardingServer
	jwks       jwksServiceServer
}

// NewService 创建 authn gRPC 服务
func NewService(
	tokenSvc tokenApp.TokenApplicationService,
	accountOnboarder onboardingApp.AccountOnboarder,
	keyPublish *jwksApp.KeyPublishAppService,
) *Service {
	return &Service{
		auth: authServiceServer{
			tokenSvc: tokenSvc,
		},
		onboarding: accountOnboardingServer{
			accountOnboarder: accountOnboarder,
		},
		jwks: jwksServiceServer{
			keyPublish: keyPublish,
		},
	}
}

// Register 注册 gRPC 服务
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	if s.auth.tokenSvc != nil {
		authnv1.RegisterAuthServiceServer(server, &s.auth)
	}
	if s.onboarding.accountOnboarder != nil {
		authnv1.RegisterAccountOnboardingServiceServer(server, &s.onboarding)
	}
	if s.jwks.keyPublish != nil {
		authnv1.RegisterJWKSServiceServer(server, &s.jwks)
	}
}

type authServiceServer struct {
	authnv1.UnimplementedAuthServiceServer
	tokenSvc tokenApp.TokenApplicationService
}

type accountOnboardingServer struct {
	authnv1.UnimplementedAccountOnboardingServiceServer
	accountOnboarder onboardingApp.AccountOnboarder
}

type jwksServiceServer struct {
	authnv1.UnimplementedJWKSServiceServer
	keyPublish *jwksApp.KeyPublishAppService
}

func (s *authServiceServer) VerifyToken(ctx context.Context, req *authnv1.VerifyTokenRequest) (*authnv1.VerifyTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAccessToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	result, err := s.tokenSvc.VerifyToken(ctx, tokenApp.VerifyTokenRequest{
		AccessToken:      req.GetAccessToken(),
		ExpectedIssuer:   strings.TrimSpace(req.GetExpectedIssuer()),
		ExpectedAudience: cloneAudience(req.GetExpectedAudience()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &authnv1.VerifyTokenResponse{}
	if result != nil {
		resp.Valid = result.Valid
	}
	if resp.Valid {
		resp.Status = authnv1.TokenStatus_TOKEN_STATUS_VALID
		resp.Claims = toProtoTokenClaims(result.Claims)
		if req.GetIncludeMetadata() {
			resp.Metadata = buildTokenMetadata(result.Claims)
		}
	} else {
		resp.Status = authnv1.TokenStatus_TOKEN_STATUS_REVOKED
		resp.FailureReason = "token invalid or expired"
	}
	return resp, nil
}

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

func (s *authServiceServer) RefreshToken(ctx context.Context, req *authnv1.RefreshTokenRequest) (*authnv1.RefreshTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	result, err := s.tokenSvc.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authnv1.RefreshTokenResponse{
		TokenPair: toProtoTokenPair(result.TokenPair),
	}, nil
}

func (s *authServiceServer) RevokeToken(ctx context.Context, req *authnv1.RevokeTokenRequest) (*authnv1.RevokeTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAccessToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}
	if err := s.tokenSvc.RevokeAccessToken(ctx, req.GetAccessToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv1.RevokeTokenResponse{}, nil
}

func (s *authServiceServer) RevokeRefreshToken(ctx context.Context, req *authnv1.RevokeRefreshTokenRequest) (*authnv1.RevokeRefreshTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}
	if err := s.tokenSvc.RevokeRefreshToken(ctx, req.GetRefreshToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv1.RevokeRefreshTokenResponse{}, nil
}

func (s *authServiceServer) IssueServiceToken(ctx context.Context, req *authnv1.IssueServiceTokenRequest) (*authnv1.IssueServiceTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetSubject()) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	var ttl time.Duration
	if req.GetTtl() != nil {
		ttl = req.GetTtl().AsDuration()
		if ttl < 0 {
			return nil, status.Error(codes.InvalidArgument, "ttl must be non-negative")
		}
	}

	var attrs map[string]string
	if req.GetAttributes() != nil {
		attrs = structToStringMap(req.GetAttributes().AsMap())
	}

	result, err := s.tokenSvc.IssueServiceToken(ctx, tokenApp.IssueServiceTokenRequest{
		Subject:    strings.TrimSpace(req.GetSubject()),
		Audience:   cloneAudience(req.GetAudience()),
		TTL:        ttl,
		Attributes: attrs,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authnv1.IssueServiceTokenResponse{
		TokenPair: toProtoTokenPair(result.TokenPair),
	}, nil
}

func (s *jwksServiceServer) GetJWKS(ctx context.Context, req *authnv1.GetJWKSRequest) (*authnv1.GetJWKSResponse, error) {
	if s.keyPublish == nil {
		return nil, status.Error(codes.Unimplemented, "jwks service not configured")
	}
	_ = req // reserved for future cache validation
	result, err := s.keyPublish.BuildJWKS(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv1.GetJWKSResponse{
		Jwks:         result.JWKS,
		Etag:         result.ETag,
		LastModified: timestamppb.New(result.LastModified),
	}, nil
}

func toProtoTokenPair(pair *tokenApp.TokenPair) *authnv1.TokenPair {
	if pair == nil || pair.AccessToken == nil {
		return nil
	}
	resp := &authnv1.TokenPair{
		TokenType:    "Bearer",
		AccessToken:  pair.AccessToken.Value,
		RefreshToken: "",
		ExpiresIn:    durationpb.New(durationUntil(pair.AccessToken.ExpiresAt)),
	}
	if pair.RefreshToken != nil {
		resp.RefreshToken = pair.RefreshToken.Value
	}
	return resp
}

func toProtoTokenClaims(claims *tokenApp.TokenClaims) *authnv1.TokenClaims {
	if claims == nil {
		return nil
	}
	resp := &authnv1.TokenClaims{
		TokenId:    claims.TokenID,
		Subject:    claims.Subject,
		Issuer:     claims.Issuer,
		Audience:   cloneAudience(claims.Audience),
		Attributes: cloneAttributes(claims.Attributes),
		Amr:        cloneAudience(claims.AMR),
		IssuedAt:   timestamppb.New(claims.IssuedAt),
		ExpiresAt:  timestamppb.New(claims.ExpiresAt),
	}
	if claims.SessionID != "" {
		resp.SessionId = claims.SessionID
	}
	if !claims.UserID.IsZero() {
		resp.UserId = claims.UserID.String()
	}
	if !claims.AccountID.IsZero() {
		resp.AccountId = claims.AccountID.String()
	}
	if !claims.TenantID.IsZero() {
		resp.TenantId = claims.TenantID.String()
	}
	return resp
}

func buildTokenMetadata(claims *tokenApp.TokenClaims) *authnv1.TokenMetadata {
	if claims == nil {
		return nil
	}
	tokenType := authnv1.TokenType_TOKEN_TYPE_ACCESS
	if claims.TokenType == tokenApp.TokenTypeService {
		tokenType = authnv1.TokenType_TOKEN_TYPE_SERVICE
	}
	return &authnv1.TokenMetadata{
		TokenType: tokenType,
		Status:    authnv1.TokenStatus_TOKEN_STATUS_VALID,
		IssuedAt:  timestamppb.New(claims.IssuedAt),
		ExpiresAt: timestamppb.New(claims.ExpiresAt),
	}
}

func durationUntil(t time.Time) time.Duration {
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

func parseOptionalMetaID(text string) (meta.ID, error) {
	if text == "" {
		return meta.ZeroID, nil
	}
	return meta.ParseID(text)
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if coder := errors.ParseCoder(err); coder != nil {
		switch coder.HTTPStatus() {
		case 400:
			return status.Error(codes.InvalidArgument, coder.String())
		case 401:
			return status.Error(codes.Unauthenticated, coder.String())
		case 403:
			return status.Error(codes.PermissionDenied, coder.String())
		case 404:
			return status.Error(codes.NotFound, coder.String())
		}
		return status.Error(codes.Internal, coder.String())
	}
	return status.Error(codes.Internal, err.Error())
}

func cloneAudience(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneAttributes(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func structToStringMap(s map[string]any) map[string]string {
	if len(s) == 0 {
		return nil
	}
	out := make(map[string]string, len(s))
	for k, v := range s {
		out[k] = fmt.Sprint(v)
	}
	return out
}
