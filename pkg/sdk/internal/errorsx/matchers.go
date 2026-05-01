package errorsx

import (
	sdkerrors "github.com/FangcunMount/iam/v2/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

type ErrorMatcher interface {
	Match(err error) bool
}

type CodeMatcher struct {
	codes []codes.Code
}

func NewCodeMatcher(c ...codes.Code) *CodeMatcher {
	return &CodeMatcher{codes: c}
}

func (m *CodeMatcher) Match(err error) bool {
	code := sdkerrors.GRPCCode(err)
	for _, c := range m.codes {
		if c == code {
			return true
		}
	}
	return false
}

type CategoryMatcher struct {
	categories []ErrorCategory
}

func NewCategoryMatcher(c ...ErrorCategory) *CategoryMatcher {
	return &CategoryMatcher{categories: c}
}

func (m *CategoryMatcher) Match(err error) bool {
	category := categorize(sdkerrors.GRPCCode(err))
	for _, c := range m.categories {
		if c == category {
			return true
		}
	}
	return false
}

var (
	AuthErrors       = NewCodeMatcher(codes.Unauthenticated, codes.PermissionDenied)
	NetworkErrors    = NewCodeMatcher(codes.Unavailable, codes.DeadlineExceeded, codes.Canceled)
	ServerErrors     = NewCodeMatcher(codes.Internal, codes.Unknown, codes.DataLoss, codes.Unimplemented)
	ValidationErrors = NewCodeMatcher(codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange)
	ResourceErrors   = NewCodeMatcher(codes.NotFound, codes.AlreadyExists)
	RetryableErrors  = NewCodeMatcher(codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.DeadlineExceeded)
)
