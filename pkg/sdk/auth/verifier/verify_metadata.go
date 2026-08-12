package verifier

import authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"

func buildVerifyMetadataFromProto(metadata *authnv2.TokenMetadata) *VerifyMetadata {
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
		Status:    authnv2.TokenStatus_TOKEN_STATUS_VALID,
		IssuedAt:  claims.IssuedAt,
		ExpiresAt: claims.ExpiresAt,
	}
}

func tokenTypeToProto(tokenType string) authnv2.TokenType {
	switch tokenType {
	case "refresh":
		return authnv2.TokenType_TOKEN_TYPE_REFRESH
	case "service":
		return authnv2.TokenType_TOKEN_TYPE_SERVICE
	case "", "access":
		return authnv2.TokenType_TOKEN_TYPE_ACCESS
	default:
		return authnv2.TokenType_TOKEN_TYPE_UNSPECIFIED
	}
}
