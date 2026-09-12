// Package operationscatalog installs only the reviewed operations read contract.
package operationscatalog

import (
	"context"
	"fmt"
	grant "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	database "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ResourceKey = "qs:statistics:collection:operations"

var roles = []string{"qs:assessment_operator", "qs:evaluation_plan_manager", "qs:result_reviewer"}

type Report struct {
	State          string   `json:"state"`
	Fingerprint    string   `json:"fingerprint"`
	BeforeHash     string   `json:"before_hash,omitempty"`
	PolicyVersion  int64    `json:"policy_version"`
	ResourceID     string   `json:"resource_id,omitempty"`
	CreateResource bool     `json:"create_resource"`
	MissingRoles   []string `json:"missing_roles"`
	GrantIDs       []string `json:"grant_ids"`
}

func inspect(s rolemodel.State) (Report, error) {
	r := Report{State: "pending", Fingerprint: s.Hash(), PolicyVersion: s.PolicyVersion, CreateResource: true, MissingRoles: []string{}, GrantIDs: []string{}}
	var catalog *resource.Resource
	for _, row := range s.Resources {
		if row.Key != ResourceKey {
			continue
		}
		if row.DeletedAt != nil {
			return r, fmt.Errorf("operations resource is archived")
		}
		bo, e := resourcepo.NewMapper().ToBO(&row)
		if e != nil {
			return r, e
		}
		if len(bo.Actions) != 1 || !bo.HasAction("read") {
			return r, fmt.Errorf("operations resource contract conflicts")
		}
		catalog = bo
		r.ResourceID = row.ID.String()
		r.CreateResource = false
	}
	for _, name := range roles {
		var id meta.ID
		for _, role := range s.Roles {
			if role.Name == name && role.DeletedAt == nil {
				if !id.IsZero() || role.ManagementProtection != "standard" {
					return r, fmt.Errorf("role conflict: %s", name)
				}
				id = role.ID
			}
		}
		if id.IsZero() {
			return r, fmt.Errorf("required standard role missing: %s", name)
		}
		found := false
		for _, row := range s.Grants {
			if row.RoleID != id.Uint64() || row.ResourcePattern != ResourceKey || row.RevokedAt != nil || row.DeletedAt != nil {
				continue
			}
			bo, e := (grantpo.Mapper{}).ToBO(&row)
			if e != nil {
				return r, e
			}
			if catalog == nil || bo.Action.String() != "read" {
				return r, fmt.Errorf("operations grant conflicts")
			}
			if e = bo.ValidateAgainst(*catalog); e != nil {
				return r, e
			}
			if found {
				return r, fmt.Errorf("duplicate operations grant")
			}
			found = true
			r.GrantIDs = append(r.GrantIDs, row.ID.String())
		}
		if !found {
			r.MissingRoles = append(r.MissingRoles, name)
		}
	}
	if !r.CreateResource && len(r.MissingRoles) == 0 {
		r.State = "configured"
	}
	return r, nil
}
func Preflight(ctx context.Context, db *gorm.DB) (Report, error) {
	s, e := rolemodel.LoadState(ctx, db)
	if e != nil {
		return Report{}, e
	}
	return inspect(s)
}

// Apply is a single local transaction, serialized with all policy mutations.
// No Assignment or Scope row is written; existing equivalent grants are reused.
func Apply(ctx context.Context, db *gorm.DB, stager event.Stager, fingerprint, actorID string, stopped bool) (Report, error) {
	actor, e := meta.ParseID(actorID)
	if e != nil || actor.IsZero() || actor.String() != actorID || !stopped || len(fingerprint) != 64 || stager == nil {
		return Report{}, fmt.Errorf("reviewed fingerprint, actor and paused authorization writes required")
	}
	ctx = context.WithValue(ctx, requestctx.KeyUserID, actor)
	var result Report
	e = database.NewUnitOfWork(db).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := database.RequireTx(txCtx)
		if err != nil {
			return err
		}
		var lock struct{ ID uint64 }
		if err = tx.Table("authz_policy_versions").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=1").Take(&lock).Error; err != nil {
			return err
		}
		before, err := rolemodel.LoadState(txCtx, tx)
		if err != nil {
			return err
		}
		plan, err := inspect(before)
		if err != nil {
			return err
		}
		if plan.Fingerprint != fingerprint {
			return fmt.Errorf("authorization facts changed; repeat preflight")
		}
		if plan.State == "configured" {
			result = plan
			return nil
		}
		var protected int64
		err = tx.Table("authz_assignments a").Joins("JOIN authz_roles r ON r.id=a.role_id").Joins("JOIN users u ON u.id=a.subject_id").Where("a.subject_type='user' AND a.subject_id=? AND a.deleted_at IS NULL AND r.deleted_at IS NULL AND r.name='platform_admin' AND r.management_protection='protected' AND u.deleted_at IS NULL AND u.status=1", actorID).Count(&protected).Error
		if err != nil {
			return err
		}
		if protected != 1 {
			return fmt.Errorf("active protected platform administrator required")
		}
		var catalog *resource.Resource
		if plan.CreateResource {
			bo, err := resource.NewResource(ResourceKey, []string{"read"}, resource.WithDisplayName("门店运营统计"), resource.WithDescription("授权门店范围内的服务人数、答卷提交和首次测评完成计数"))
			if err != nil {
				return err
			}
			row := resourcepo.NewMapper().ToPO(&bo)
			if err = tx.Create(row).Error; err != nil {
				return err
			}
			catalog, err = resourcepo.NewMapper().ToBO(row)
			if err != nil {
				return err
			}
		} else {
			for _, row := range before.Resources {
				if row.ID.String() == plan.ResourceID {
					catalog, err = resourcepo.NewMapper().ToBO(&row)
					if err != nil {
						return err
					}
				}
			}
		}
		if catalog == nil {
			return fmt.Errorf("operations resource missing")
		}
		for _, name := range plan.MissingRoles {
			for _, role := range before.Roles {
				if role.Name != name || role.DeletedAt != nil {
					continue
				}
				bo, err := grant.New(role.ID, catalog.ID, ResourceKey, "read", "user:"+actorID)
				if err != nil {
					return err
				}
				if err = bo.ValidateAgainst(*catalog); err != nil {
					return err
				}
				row, err := (grantpo.Mapper{}).ToPO(&bo)
				if err != nil {
					return err
				}
				if err = tx.Create(row).Error; err != nil {
					return err
				}
			}
		}
		v, err := policypo.NewPolicyVersionRepository(tx).Increment(txCtx, "user:"+actorID, "statistics operations read catalog")
		if err != nil {
			return err
		}
		if err = stager.Stage(txCtx, policy.NewVersionChangedEvent(v.Version)); err != nil {
			return err
		}
		after, err := rolemodel.LoadState(txCtx, tx)
		if err != nil {
			return err
		}
		result, err = inspect(after)
		result.BeforeHash = plan.Fingerprint
		return err
	})
	return result, e
}
