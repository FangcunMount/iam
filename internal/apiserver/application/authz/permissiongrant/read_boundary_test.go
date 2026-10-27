package permissiongrant_test

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	permissionGrantApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/permissiongrant"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/stretchr/testify/require"
)

// A write-only UOW must not become a fallback read boundary.
type writeOnlyBoundary struct{ calls int }

func (b *writeOnlyBoundary) WithinTx(context.Context, func(context.Context, authzuow.TxRepositories) error) error {
	b.calls++
	return nil
}

func TestPermissionGrantReadRequiresExplicitReadBoundary(t *testing.T) {
	fixture, _, _ := setupPermissionGrantService(t)
	role := seedRole(t, fixture.Roles, "test:read-boundary")
	writeOnly := &writeOnlyBoundary{}
	service := permissionGrantApp.NewService(writeOnly, fixture.PermissionGrants, nil, management.NewGuard(nil))
	items, err := service.ListByRole(context.Background(), role.ID)
	require.Error(t, err)
	require.Nil(t, items)
	require.Zero(t, writeOnly.calls, "read fell back to an unplanned write transaction")
}
