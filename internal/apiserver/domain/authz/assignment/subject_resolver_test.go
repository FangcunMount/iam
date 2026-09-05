package assignment_test

import (
	"context"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/subjectresolver"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSubjectResolverRegistryResolvesUsersAndRejectsUnsupportedSubjects(t *testing.T) {
	t.Parallel()

	userResolver := testhelpers.NewUserResolverStub(meta.FromUint64(123))
	registry := assignment.NewSubjectResolverRegistry(subjectresolver.NewUserSubjectResolver(userResolver))
	tenantID, err := tenant.NewID("tenant-a")
	require.NoError(t, err)

	userRef, err := subject.NewUserRef(meta.FromUint64(123))
	require.NoError(t, err)
	require.NoError(t, registry.Resolve(context.Background(), userRef, tenantID))

	groupRef, err := subject.NewRef(subject.TypeGroup, meta.FromUint64(99))
	require.NoError(t, err)
	err = registry.Resolve(context.Background(), groupRef, tenantID)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
	var unsupported assignment.UnsupportedSubjectTypeError
	require.ErrorAs(t, err, &unsupported)
}

func TestUserSubjectResolverReportsMissingUsers(t *testing.T) {
	t.Parallel()

	registry := assignment.NewSubjectResolverRegistry(subjectresolver.NewUserSubjectResolver(testhelpers.NewUserResolverStub()))
	tenantID, err := tenant.NewID("tenant-a")
	require.NoError(t, err)
	userRef, err := subject.NewUserRef(meta.FromUint64(404))
	require.NoError(t, err)

	err = registry.Resolve(context.Background(), userRef, tenantID)
	require.True(t, perrors.IsCode(err, code.ErrUserNotFound))
}
