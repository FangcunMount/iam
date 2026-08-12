package errorsx

import (
	stdErrors "errors"
	"time"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

type ErrorDetails struct {
	Code            string
	GRPCCode        codes.Code
	Message         string
	Category        ErrorCategory
	IsRetryable     bool
	IsClientError   bool
	IsServerError   bool
	SuggestedAction ErrorAction
	RetryAfter      time.Duration
}

func Analyze(err error) *ErrorDetails {
	if err == nil {
		return nil
	}

	details := &ErrorDetails{
		GRPCCode: sdkerrors.GRPCCode(err),
		Message:  sdkerrors.Message(err),
	}
	details.Code = details.GRPCCode.String()
	if iamErr, ok := asIAMError(err); ok && iamErr.Code != "" {
		details.Code = iamErr.Code
	}

	details.Category = categorize(details.GRPCCode)
	details.IsClientError = isClientError(details.GRPCCode)
	details.IsServerError = isServerError(details.GRPCCode)
	details.IsRetryable = isRetryableCode(details.GRPCCode)
	details.SuggestedAction = suggestAction(details.GRPCCode)

	if details.GRPCCode == codes.ResourceExhausted {
		details.RetryAfter = 5 * time.Second
	}

	return details
}

func asIAMError(err error) (*sdkerrors.IAMError, bool) {
	var iamErr *sdkerrors.IAMError
	if stdErrors.As(err, &iamErr) {
		return iamErr, true
	}
	return nil, false
}
