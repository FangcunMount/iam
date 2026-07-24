package safelog

import (
	"errors"
	"fmt"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDescribeErrorNeverIncludesErrorText(t *testing.T) {
	t.Parallel()
	sentinel := "SECRET-PHONE-TOKEN-OTP"

	for _, err := range []error{
		errors.New(sentinel),
		perrors.WithCode(100101, "failed with %s", sentinel),
		fmt.Errorf("wrapped: %w", errors.New(sentinel)),
	} {
		descriptor := fmt.Sprintf("%+v", DescribeError(err))
		require.NotContains(t, descriptor, sentinel)
	}
}
