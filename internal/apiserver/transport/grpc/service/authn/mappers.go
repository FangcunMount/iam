package authn

import (
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	tokenApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
