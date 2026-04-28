package role_test

import (
	"context"
	"testing"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

// repo stub implementing role.Repository
type roleRepoStub struct {
	found *role.Role
	err   error
}

func (r *roleRepoStub) Create(ctx context.Context, rn *role.Role) error { return nil }
func (r *roleRepoStub) Update(ctx context.Context, rn *role.Role) error { return nil }
func (r *roleRepoStub) Delete(ctx context.Context, id meta.ID) error    { return nil }
func (r *roleRepoStub) FindByID(ctx context.Context, id meta.ID) (*role.Role, error) {
	return r.found, r.err
}
func (r *roleRepoStub) FindByName(ctx context.Context, tenantID, name string) (*role.Role, error) {
	return r.found, r.err
}
func (r *roleRepoStub) List(ctx context.Context, tenantID string, offset, limit int) ([]*role.Role, int64, error) {
	return nil, 0, nil
}

func TestValidateCreateParametersAndCheckNameUnique(t *testing.T) {
	v := role.NewValidator(&roleRepoStub{})

	// missing name
	err := v.ValidateCreateParameters("", "dn", "t")
	require.Error(t, err)
	require.True(t, errors.IsCode(err, code.ErrInvalidArgument))

	// missing display name
	err = v.ValidateCreateParameters("n", "", "t")
	require.Error(t, err)

	// missing tenant
	err = v.ValidateCreateParameters("n", "dn", "")
	require.Error(t, err)

	// CheckNameUnique: tenant empty
	err = v.CheckNameUnique(context.Background(), "", "n")
	require.Error(t, err)

	// name empty
	err = v.CheckNameUnique(context.Background(), "t", "")
	require.Error(t, err)

	// repo returns existing role -> ErrRoleAlreadyExists
	stub := &roleRepoStub{found: &role.Role{ID: meta.FromUint64(1), Name: "n", TenantID: "t"}, err: nil}
	v2 := role.NewValidator(stub)
	err = v2.CheckNameUnique(context.Background(), "t", "n")
	require.Error(t, err)
	require.True(t, errors.IsCode(err, code.ErrRoleAlreadyExists))

	// repo returns not found -> ok
	stub2 := &roleRepoStub{found: nil, err: errors.WithCode(code.ErrRoleNotFound, "nf")}
	v3 := role.NewValidator(stub2)
	err = v3.CheckNameUnique(context.Background(), "t", "nx")
	require.NoError(t, err)

	// repo returns other error -> wrapped
	stub3 := &roleRepoStub{found: nil, err: errors.New("db")}
	v4 := role.NewValidator(stub3)
	err = v4.CheckNameUnique(context.Background(), "t", "nx")
	require.Error(t, err)
}

func TestCheckRoleExistsAndTenantOwnership(t *testing.T) {
	v := role.NewValidator(&roleRepoStub{})

	// id zero
	_, err := v.CheckRoleExists(context.Background(), meta.ID(0))
	require.Error(t, err)

	// not found
	stub := &roleRepoStub{found: nil, err: errors.WithCode(code.ErrRoleNotFound, "nf")}
	v2 := role.NewValidator(stub)
	_, err = v2.CheckRoleExists(context.Background(), meta.FromUint64(10))
	require.Error(t, err)
	require.True(t, errors.IsCode(err, code.ErrRoleNotFound))

	// repo error
	stub2 := &roleRepoStub{found: nil, err: errors.New("db")}
	v3 := role.NewValidator(stub2)
	_, err = v3.CheckRoleExists(context.Background(), meta.FromUint64(10))
	require.Error(t, err)

	// found
	rentity := &role.Role{ID: meta.FromUint64(11), Name: "r", TenantID: "T1"}
	stub3 := &roleRepoStub{found: rentity, err: nil}
	v4 := role.NewValidator(stub3)
	got, err := v4.CheckRoleExists(context.Background(), meta.FromUint64(11))
	require.NoError(t, err)
	require.Equal(t, rentity, got)

	// CheckTenantOwnership: nil role
	err = v4.CheckTenantOwnership(nil, "T1")
	require.Error(t, err)

	// missing tenant
	err = v4.CheckTenantOwnership(rentity, "")
	require.Error(t, err)

	// mismatch
	err = v4.CheckTenantOwnership(rentity, "OTHER")
	require.Error(t, err)
	require.True(t, errors.IsCode(err, code.ErrPermissionDenied))

	// ok
	err = v4.CheckTenantOwnership(rentity, "T1")
	require.NoError(t, err)
}
