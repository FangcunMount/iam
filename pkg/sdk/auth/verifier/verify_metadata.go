package verifier

import authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"

func buildVerifyMetadataFromProto(metadata *authnv1.TokenMetadata) *VerifyMetadata {
	if metadata == nil {
		return nil
	}
	result := &VerifyMetadata{
		TokenType: metadata.GetTokenType(),
		Status:    metadata.GetStatus(),
	}
	if metadata.GetIssuedAt() != nil {
		result.IssuedAt = metadata.GetIssuedAt().AsTime()
	}
	if metadata.GetExpiresAt() != nil {
		result.ExpiresAt = metadata.GetExpiresAt().AsTime()
	}
	return result
}

func buildVerifyMetadataFromClaims(claims *TokenClaims) *VerifyMetadata {
	if claims == nil {
		return nil
	}
	return &VerifyMetadata{
		TokenType: tokenTypeToProto(claims.TokenType),
		Status:    authnv1.TokenStatus_TOKEN_STATUS_VALID,
		IssuedAt:  claims.IssuedAt,
		ExpiresAt: claims.ExpiresAt,
	}
}

func tokenTypeToProto(tokenType string) authnv1.TokenType {
	switch tokenType {
	case "refresh":
		return authnv1.TokenType_TOKEN_TYPE_REFRESH
	case "service":
		return authnv1.TokenType_TOKEN_TYPE_SERVICE
	case "", "access":
		return authnv1.TokenType_TOKEN_TYPE_ACCESS
	default:
		return authnv1.TokenType_TOKEN_TYPE_UNSPECIFIED
	}
}
