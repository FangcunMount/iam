package rolebinding_test

import (
	"testing"

	binding "github.com/FangcunMount/iam/internal/apiserver/domain/authz/rolebinding"
	"github.com/stretchr/testify/assert"
)

func TestBindingCreate(t *testing.T) {
	a := binding.NewBinding(binding.SubjectTypeUser, "u1", 42, "tenant", binding.WithID(binding.NewBindingID(5)), binding.WithGrantedBy("admin"))
	assert.Equal(t, binding.SubjectTypeUser, a.SubjectType)
	assert.Equal(t, "u1", a.SubjectID)
	assert.Equal(t, "admin", a.GrantedBy)
}
