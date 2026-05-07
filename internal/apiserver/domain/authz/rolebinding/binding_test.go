package rolebinding_test

import (
	"testing"

	binding "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestBindingCreate(t *testing.T) {
	a := binding.NewBinding(binding.SubjectTypeUser, meta.FromUint64(1), meta.FromUint64(42), "tenant", binding.WithID(binding.NewBindingID(5)), binding.WithGrantedBy("admin"))
	assert.Equal(t, binding.SubjectTypeUser, a.SubjectType)
	assert.Equal(t, meta.FromUint64(1), a.SubjectID)
	assert.Equal(t, "admin", a.GrantedBy)
}
