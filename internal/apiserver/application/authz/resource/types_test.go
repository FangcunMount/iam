package resource_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestNewUpdateResourceCommandOmitsDisplayNameWhenPointerNil(t *testing.T) {
	cmd, err := resourceApp.NewUpdateResourceCommand(resourceDomain.NewResourceID(1), nil, []string{"read"}, nil, nil)
	require.NoError(t, err)
	require.Nil(t, cmd.DisplayName)
}

func TestNewUpdateResourceCommandRejectsExplicitEmptyDisplayName(t *testing.T) {
	empty := "   "
	_, err := resourceApp.NewUpdateResourceCommand(resourceDomain.NewResourceID(1), &empty, []string{"read"}, nil, nil)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
