package roleinheritance

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Repository interface {
	// CreateChecked locks the tenant graph and validates references, cycles and depth atomically.
	CreateChecked(ctx context.Context, inheritance *Inheritance) error
	AtomicRevoke(ctx context.Context, id meta.ID, tenantID string) (RevokeOutcome, error)
	FindByID(ctx context.Context, id meta.ID) (*Inheritance, error)
	ListActiveByTenant(ctx context.Context, tenantID string) ([]*Inheritance, error)
}

func WouldCreateCycle(existing []*Inheritance, roleID, inheritedRoleID meta.ID) bool {
	if roleID.IsZero() || inheritedRoleID.IsZero() || roleID == inheritedRoleID {
		return true
	}
	graph := make(map[meta.ID][]meta.ID)
	for _, edge := range existing {
		if edge == nil || !edge.IsActive() {
			continue
		}
		graph[edge.RoleID] = append(graph[edge.RoleID], edge.InheritedRoleID)
	}
	visited := make(map[meta.ID]struct{})
	stack := []meta.ID{inheritedRoleID}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == roleID {
			return true
		}
		if _, seen := visited[current]; seen {
			continue
		}
		visited[current] = struct{}{}
		stack = append(stack, graph[current]...)
	}
	return false
}
