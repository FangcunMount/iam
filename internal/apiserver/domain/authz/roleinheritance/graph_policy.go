package roleinheritance

import (
	"fmt"
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"sort"
)

const MaxHierarchyDepth = 32

type RoleNode struct {
	ID       meta.ID
	TenantID string
}

// ValidateGraph is shared by atomic writes, runtime compilation and preflight.
// Depth counts role nodes, including the directly assigned role.
func ValidateGraph(roles []RoleNode, edges []*Inheritance) error {
	tenants := make(map[meta.ID]string, len(roles))
	indegree := make(map[meta.ID]int, len(roles))
	depth := make(map[meta.ID]int, len(roles))
	previous := make(map[meta.ID]meta.ID)
	graph := make(map[meta.ID][]meta.ID)
	for _, node := range roles {
		if node.ID.IsZero() || node.TenantID == "" {
			return invalidGraph("invalid role node %s", node.ID)
		}
		if _, exists := tenants[node.ID]; exists {
			return invalidGraph("duplicate role %s", node.ID)
		}
		tenants[node.ID] = node.TenantID
		indegree[node.ID] = 0
		depth[node.ID] = 1
	}
	for _, edge := range edges {
		if edge == nil {
			return invalidGraph("nil inheritance")
		}
		if !edge.IsActive() {
			continue
		}
		child, childOK := tenants[edge.RoleID]
		parent, parentOK := tenants[edge.InheritedRoleID]
		if !childOK || !parentOK || child != edge.TenantIDString() || parent != child {
			return invalidGraph("unknown or cross-tenant role in edge %s -> %s", edge.RoleID, edge.InheritedRoleID)
		}
		graph[edge.RoleID] = append(graph[edge.RoleID], edge.InheritedRoleID)
		indegree[edge.InheritedRoleID]++
	}
	queue := make([]meta.ID, 0, len(roles))
	for id, count := range indegree {
		if count == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	for index := 0; index < len(queue); index++ {
		child := queue[index]
		for _, parent := range graph[child] {
			if depth[child]+1 > depth[parent] {
				depth[parent] = depth[child] + 1
				previous[parent] = child
			}
			if depth[parent] > MaxHierarchyDepth {
				path := []meta.ID{parent}
				for n := previous[parent]; !n.IsZero(); n = previous[n] {
					path = append(path, n)
				}
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return invalidGraph("role inheritance exceeds %d nodes: %v", MaxHierarchyDepth, path)
			}
			indegree[parent]--
			if indegree[parent] == 0 {
				queue = append(queue, parent)
			}
		}
	}
	if len(queue) != len(roles) {
		cyclic := make([]meta.ID, 0)
		for id, count := range indegree {
			if count > 0 {
				cyclic = append(cyclic, id)
			}
		}
		sort.Slice(cyclic, func(i, j int) bool { return cyclic[i] < cyclic[j] })
		return invalidGraph("role inheritance contains a cycle involving %v", cyclic)
	}
	return nil
}
func invalidGraph(format string, args ...any) error {
	return perrors.WithCode(code.ErrInvalidArgument, "%s", fmt.Sprintf(format, args...))
}
