package resource

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/stretchr/testify/assert"
)

func TestResourceAndActions(t *testing.T) {
	r := NewResource("scale:form:*", []string{"read", "write"}, WithDisplayName("Form"), WithAppName("scale"))
	assert.Equal(t, "scale:form:*", r.Key)
	assert.True(t, r.HasAction("read"))
	assert.False(t, r.HasAction("delete"))
}

func TestResourceDomainBehavior(t *testing.T) {
	originScope, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	assert.NoError(t, err)

	r := NewResource(
		"iam:users",
		[]string{"read"},
		WithScopeKinds([]authzDomain.ScopeKind{authzDomain.ScopeKindAll, authzDomain.ScopeKindOrigin}),
	)
	assert.True(t, r.Supports("read", authzDomain.DefaultScope()))
	assert.True(t, r.Supports("read", originScope))
	assert.False(t, r.Supports("update", originScope))

	err = r.ChangeCatalog([]string{"update"}, []authzDomain.ScopeKind{authzDomain.ScopeKindAll})
	assert.NoError(t, err)
	assert.True(t, r.Supports("update", authzDomain.DefaultScope()))
	assert.False(t, r.Supports("read", authzDomain.DefaultScope()))

	err = r.ChangeCatalog(nil, nil)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
