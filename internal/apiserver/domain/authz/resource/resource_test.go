package resource

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/assert"
)

func TestResourceAndActions(t *testing.T) {
	r, err := NewResource("scale:form:template:*", []string{"read", "write"}, WithDisplayName("Form"), WithAppName("scale"))
	assert.NoError(t, err)
	assert.Equal(t, "scale:form:template:*", r.KeyString())
	assert.Equal(t, []string{"read", "write"}, r.ActionStrings())
	assert.True(t, r.HasAction("read"))
	assert.False(t, r.HasAction("delete"))
}

func TestResourceDomainBehavior(t *testing.T) {
	originScope, err := scope.New(scope.KindOrigin, "1")
	assert.NoError(t, err)

	r, err := NewResource(
		"iam:identity:collection:users",
		[]string{"read"},
		WithScopeKinds([]scope.Kind{scope.KindAll, scope.KindOrigin}),
	)
	assert.NoError(t, err)
	assert.True(t, r.Supports("read", scope.Default()))
	assert.True(t, r.Supports("read", originScope))
	assert.False(t, r.Supports("update", originScope))

	err = r.ChangeCatalog([]string{"update"}, []scope.Kind{scope.KindAll})
	assert.NoError(t, err)
	assert.True(t, r.Supports("update", scope.Default()))
	assert.False(t, r.Supports("read", scope.Default()))

	err = r.ChangeCatalog(nil, nil)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestResourceRejectsLegacyTwoSegmentKey(t *testing.T) {
	_, err := NewResource("iam:users", []string{"read"})
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestResourceKeyAndPatternHaveDifferentWildcardSemantics(t *testing.T) {
	key, err := NewKey("scale:form:template:*")
	assert.NoError(t, err)
	assert.Equal(t, "scale:form:template:*", key.String())

	_, err = NewKey("*:*:*:*")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewKey("qs:*:*:*")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	pattern, err := NewPattern("*:*:*:*")
	assert.NoError(t, err)
	assert.Equal(t, "*:*:*:*", pattern.String())

	pattern, err = NewPattern("qs:*:*:*")
	assert.NoError(t, err)
	assert.Equal(t, "qs", pattern.App())
}

func TestResourceRejectsUnsupportedScopeKindInsideEntity(t *testing.T) {
	_, err := NewResource(
		"iam:identity:collection:users",
		[]string{"read"},
		WithScopeKinds([]scope.Kind{"project"}),
	)
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	r, err := NewResource("iam:identity:collection:users", []string{"read"})
	assert.NoError(t, err)
	err = r.ChangeCatalog([]string{"read"}, []scope.Kind{"project"})
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestActionAndActionPatternHaveDifferentSemantics(t *testing.T) {
	action, err := NewAction(" read ")
	assert.NoError(t, err)
	assert.Equal(t, "read", action.String())

	_, err = NewAction("read|list")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewAction(".*")
	assert.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	pattern, err := NewActionPattern(" read|list ")
	assert.NoError(t, err)
	assert.Equal(t, "read|list", pattern.String())

	pattern, err = NewActionPattern(".*")
	assert.NoError(t, err)
	assert.Equal(t, ".*", pattern.String())
}
