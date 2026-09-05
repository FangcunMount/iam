package permissiongrant

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

const WildcardAction = "*"

type Grant struct {
	ID              meta.ID
	TenantID        tenant.ID
	RoleID          meta.ID
	ResourceID      resource.ResourceID
	ResourcePattern resource.Pattern
	Action          resource.ActionPattern
	Constraints     constraint.Set
	GrantKey        string
	GrantedBy       string
	GrantedAt       time.Time
	RevokedAt       *time.Time
	Version         uint32
}

func New(
	roleID meta.ID,
	tenantID string,
	resourceID resource.ResourceID,
	resourcePattern string,
	action string,
	constraints constraint.Set,
	grantedBy string,
) (Grant, error) {
	if resourceID.Uint64() == 0 {
		return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "resource id is required for managed permission grant")
	}
	concreteAction, err := resource.NewAction(action)
	if err != nil {
		return Grant{}, err
	}
	return newGrant(roleID, tenantID, resourceID, resourcePattern, concreteAction.String(), constraints, grantedBy, false)
}

// NewSystem creates a trusted bootstrap/migration grant. It is the only path
// that accepts a resource pattern without a catalog resource ID or wildcard
// action, and it never accepts conditional constraints.
func NewSystem(
	roleID meta.ID,
	tenantID string,
	resourceID resource.ResourceID,
	resourcePattern string,
	action string,
	constraints constraint.Set,
	grantedBy string,
) (Grant, error) {
	return newGrant(roleID, tenantID, resourceID, resourcePattern, action, constraints, grantedBy, true)
}

func newGrant(
	roleID meta.ID,
	tenantID string,
	resourceID resource.ResourceID,
	resourcePattern string,
	action string,
	constraints constraint.Set,
	grantedBy string,
	system bool,
) (Grant, error) {
	if roleID.IsZero() {
		return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "role id is required")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Grant{}, err
	}
	pattern, err := resource.NewPattern(resourcePattern)
	if err != nil {
		return Grant{}, err
	}
	constraints, err = constraints.Normalize()
	if err != nil {
		return Grant{}, err
	}
	action = strings.TrimSpace(action)
	if system {
		if !constraints.IsUnconditional() {
			return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "system wildcard grants must be unconditional")
		}
		if action != WildcardAction {
			concrete, concreteErr := resource.NewAction(action)
			if concreteErr != nil {
				return Grant{}, concreteErr
			}
			action = concrete.String()
		}
	} else {
		if resourceID.Uint64() == 0 {
			return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "conditional and managed grants require a catalog resource")
		}
		concrete, concreteErr := resource.NewAction(action)
		if concreteErr != nil {
			return Grant{}, concreteErr
		}
		action = concrete.String()
	}
	if !constraints.IsUnconditional() && isBulkAction(action) {
		return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "conditional grants cannot authorize list, search, or batch actions")
	}
	actionPattern, err := resource.NewActionPattern(action)
	if err != nil {
		return Grant{}, err
	}
	grantedBy = strings.TrimSpace(grantedBy)
	if grantedBy == "" {
		return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "granted by is required")
	}
	grant := Grant{
		TenantID:        tenantIDValue,
		RoleID:          roleID,
		ResourceID:      resourceID,
		ResourcePattern: pattern,
		Action:          actionPattern,
		Constraints:     constraints,
		GrantedBy:       grantedBy,
		Version:         1,
	}
	grant.GrantKey, err = grant.computeKey()
	if err != nil {
		return Grant{}, err
	}
	return grant, nil
}

func isBulkAction(action string) bool {
	return action == "list" || action == "search" || action == "batch" || strings.HasPrefix(action, "batch_")
}

func (g Grant) IsActive() bool { return g.RevokedAt == nil }

func (g Grant) IsConditional() bool { return !g.Constraints.IsUnconditional() }

// ValidateAgainst binds a managed grant to the resource catalog contract.
// Trusted wildcard grants have no catalog resource and are unconditional by
// construction.
func (g Grant) ValidateAgainst(catalogResource resource.Resource) error {
	if g.ResourceID.Uint64() == 0 {
		if !g.Constraints.IsUnconditional() {
			return perrors.WithCode(code.ErrInvalidArgument, "wildcard permission grant must be unconditional")
		}
		return nil
	}
	if catalogResource.ID.Uint64() != g.ResourceID.Uint64() {
		return perrors.WithCode(code.ErrInvalidArgument, "permission grant resource does not match catalog resource")
	}
	if catalogResource.KeyString() != g.ResourcePatternString() {
		return perrors.WithCode(code.ErrInvalidArgument, "permission grant resource pattern must equal catalog resource key")
	}
	if !catalogResource.HasAction(g.ActionString()) {
		return perrors.WithCode(code.ErrInvalidArgument, "permission grant action is not registered by resource")
	}
	return g.Constraints.ValidateAgainst(catalogResource.AttributeSchema)
}

func (g Grant) Evaluate(attributes constraint.Attributes) (constraint.Evaluation, error) {
	return g.Constraints.Evaluate(attributes)
}

func (g Grant) MatchesAction(action resource.Action) bool {
	return g.Action.String() == WildcardAction || g.Action.Matches(action)
}

func (g Grant) CoversResource(candidate resource.Pattern) bool {
	return g.ResourcePattern.Covers(candidate)
}

func (g *Grant) Revoke(at time.Time) error {
	if g == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "permission grant is required")
	}
	if g.RevokedAt != nil {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	g.RevokedAt = &at
	return nil
}

func (g Grant) TenantIDString() string        { return g.TenantID.String() }
func (g Grant) ResourcePatternString() string { return g.ResourcePattern.String() }
func (g Grant) ActionString() string          { return g.Action.String() }

func (g Grant) CanonicalConstraintJSON() ([]byte, error) {
	return g.Constraints.CanonicalJSON()
}

func (g Grant) computeKey() (string, error) {
	constraints, err := g.CanonicalConstraintJSON()
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf("v1\x00%s\x00%d\x00%d\x00%s\x00%s\x00%s",
		g.TenantIDString(),
		g.RoleID.Uint64(),
		g.ResourceID.Uint64(),
		g.ResourcePatternString(),
		g.ActionString(),
		constraints,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:]), nil
}

type RestoreOptions struct {
	ID        meta.ID
	GrantKey  string
	GrantedAt time.Time
	RevokedAt *time.Time
	Version   uint32
}

func Restore(
	roleID meta.ID,
	tenantID string,
	resourceID resource.ResourceID,
	resourcePattern string,
	action string,
	constraints constraint.Set,
	grantedBy string,
	options RestoreOptions,
) (Grant, error) {
	system := resourceID.Uint64() == 0 || action == WildcardAction
	grant, err := newGrant(roleID, tenantID, resourceID, resourcePattern, action, constraints, grantedBy, system)
	if err != nil {
		return Grant{}, err
	}
	if options.GrantKey != "" && options.GrantKey != grant.GrantKey {
		return Grant{}, perrors.WithCode(code.ErrInvalidArgument, "permission grant canonical key mismatch")
	}
	grant.ID = options.ID
	grant.GrantedAt = options.GrantedAt
	grant.RevokedAt = options.RevokedAt
	if options.Version > 0 {
		grant.Version = options.Version
	}
	return grant, nil
}

func (g Grant) Clone() Grant {
	out := g
	out.Constraints = g.Constraints.Clone()
	if g.RevokedAt != nil {
		value := *g.RevokedAt
		out.RevokedAt = &value
	}
	return out
}
