package authn

import (
	"fmt"
	"strings"
	"time"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	onboardingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	tokenApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	iamgrpc "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoTokenPair(pair *tokenApp.TokenPair) *authnv2.TokenPair {
	if pair == nil || pair.AccessToken == nil {
		return nil
	}
	resp := &authnv2.TokenPair{
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

func toProtoTokenClaims(claims *tokenApp.TokenClaims) *authnv2.TokenClaims {
	if claims == nil {
		return nil
	}
	resp := &authnv2.TokenClaims{
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
	if !claims.LoginIdentityID.IsZero() {
		resp.LoginIdentityId = claims.LoginIdentityID.String()
	}
	if domain := claims.TenantDomain; domain != "" {
		resp.TenantId = domain
	}
	if !claims.OrgID.IsZero() {
		resp.OrgId = claims.OrgID.String()
	}
	return resp
}

func buildTokenMetadata(claims *tokenApp.TokenClaims) *authnv2.TokenMetadata {
	if claims == nil {
		return nil
	}
	tokenType := authnv2.TokenType_TOKEN_TYPE_ACCESS
	if claims.TokenType == tokenApp.TokenTypeService {
		tokenType = authnv2.TokenType_TOKEN_TYPE_SERVICE
	}
	return &authnv2.TokenMetadata{
		TokenType: tokenType,
		Status:    authnv2.TokenStatus_TOKEN_STATUS_VALID,
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

func parseRequiredMetaID(text, field string) (meta.ID, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return meta.ZeroID, fmt.Errorf("%s is required", field)
	}
	id, err := meta.ParseID(text)
	if err != nil {
		return meta.ZeroID, fmt.Errorf("invalid %s: %w", field, err)
	}
	if id.IsZero() {
		return meta.ZeroID, fmt.Errorf("%s is required", field)
	}
	return id, nil
}

func parseAuthenticatedUserContext(actor *authnv2.AuthenticatedUserContext) (meta.ID, meta.ID, *time.Time, error) {
	if actor == nil {
		return meta.ZeroID, meta.ZeroID, nil, fmt.Errorf("actor is required")
	}
	userID, err := parseRequiredMetaID(actor.GetUserId(), "actor.user_id")
	if err != nil {
		return meta.ZeroID, meta.ZeroID, nil, err
	}
	currentID, err := parseOptionalMetaID(strings.TrimSpace(actor.GetCurrentLoginIdentityId()))
	if err != nil {
		return meta.ZeroID, meta.ZeroID, nil, fmt.Errorf("invalid actor.current_login_identity_id: %w", err)
	}
	var authenticatedAt *time.Time
	if ts := actor.GetAuthenticatedAt(); ts != nil {
		t := ts.AsTime()
		authenticatedAt = &t
	}
	return userID, currentID, authenticatedAt, nil
}

func toProtoSignupResult(result *onboardingApp.OnboardingResult) *authnv2.SignupResult {
	if result == nil {
		return &authnv2.SignupResult{}
	}
	return &authnv2.SignupResult{
		UserId:          result.UserID.String(),
		UserName:        result.UserName,
		Phone:           result.Phone.String(),
		Email:           result.Email.String(),
		LoginIdentityId: result.LoginIdentityID.String(),
		Credential:      toProtoSignupCredential(result.Credential),
		IsNewUser:       result.IsNewUser,
		IsNewIdentity:   result.IsNewLoginIdentity,
	}
}

func toProtoSignupCredential(credential *onboardingApp.OnboardingCredential) *authnv2.SignupCredential {
	if credential == nil {
		return nil
	}
	return &authnv2.SignupCredential{
		Id:   credential.ID.String(),
		Type: credentialTypeString(credential.Type),
	}
}

func toProtoLoginIdentityView(identity linkingApp.LoginIdentityView) *authnv2.LoginIdentity {
	return &authnv2.LoginIdentity{
		Id:               identity.ID.String(),
		UserId:           identity.UserID.String(),
		Provider:         string(identity.Provider),
		Realm:            identity.Realm,
		Identifier:       identity.Identifier,
		GlobalIdentifier: identity.GlobalIdentifier,
		Status:           string(identity.Status),
		VerifiedAt:       optionalTimestamp(identity.VerifiedAt),
		LinkedAt:         timestamppb.New(identity.LinkedAt),
	}
}

func toProtoLinkResult(result *linkingApp.LinkResult) *authnv2.LinkLoginIdentityResponse {
	if result == nil || result.Identity == nil {
		return &authnv2.LinkLoginIdentityResponse{}
	}
	return &authnv2.LinkLoginIdentityResponse{
		LoginIdentity: toProtoLoginIdentityView(linkingApp.LoginIdentityView{
			ID:               result.Identity.ID,
			UserID:           result.Identity.UserID,
			Provider:         result.Identity.Provider,
			Realm:            result.Identity.Realm,
			Identifier:       result.Identity.Identifier,
			GlobalIdentifier: result.Identity.GlobalIdentifier,
			Status:           result.Identity.Status,
			VerifiedAt:       result.Identity.VerifiedAt,
			LinkedAt:         result.Identity.LinkedAt,
		}),
		Reused: result.Reused,
	}
}

func optionalTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

func toGRPCError(err error) error {
	return iamgrpc.ToStatusError(err)
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

func credentialTypeString(typ credDomain.CredentialType) string {
	if typ == "" {
		return ""
	}
	return string(typ)
}
