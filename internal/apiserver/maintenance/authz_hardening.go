package maintenance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	grantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	inheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	dbmysql "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const AuthzHardeningConfirmation = "APPLY_AUTHZ_HARDENING"
const hardeningActor = "iam-maintenance:authz-hardening"

type HardeningIssue struct {
	Kind   string `json:"kind"`
	Object string `json:"object"`
	Detail string `json:"detail"`
}
type HardeningChange struct {
	Grant              *permissiongrant.Grant `json:"grant"`
	ReplacementActions []string               `json:"replacement_actions,omitempty"`
}
type AuthzHardeningReport struct {
	Fingerprint         string            `json:"fingerprint"`
	ProviderFingerprint string            `json:"provider_fingerprint"`
	Blockers            []HardeningIssue  `json:"blockers"`
	Changes             []HardeningChange `json:"changes"`
	PlatformWriters     []string          `json:"platform_writers"`
	PolicyVersions      map[string]int64  `json:"policy_versions"`
	BeforeFingerprint   string            `json:"before_fingerprint,omitempty"`
	AppliedChanges      []HardeningChange `json:"applied_changes,omitempty"`
	Complete            bool              `json:"complete"`
}
type hardeningUser struct {
	ID        meta.ID
	Status    int
	DeletedAt *time.Time
}
type hardeningState struct {
	Roles               []*rolerepo.RolePO
	Resources           []*resourcerepo.ResourcePO
	Assignments         []*assignmentrepo.AssignmentPO
	Inheritances        []*inheritancerepo.InheritancePO
	Grants              []*grantrepo.GrantPO
	Versions            []*policyrepo.PolicyVersionPO
	Users               []hardeningUser
	ProviderFingerprint string
}

func loadHardeningState(ctx context.Context, db *gorm.DB, locked bool, providers *objectattributeadmission.Registry) (*hardeningState, error) {
	if providers == nil {
		return nil, fmt.Errorf("attribute provider configuration required")
	}
	s := &hardeningState{ProviderFingerprint: providers.Fingerprint()}
	// Lock full ordered ranges in apply, including history, so normal writers and
	// concurrent inserts cannot invalidate the reviewed fingerprint mid-transaction.
	for _, rows := range []any{&s.Roles, &s.Resources, &s.Assignments, &s.Inheritances, &s.Grants, &s.Versions} {
		q := db.WithContext(ctx).Unscoped().Order("id ASC")
		if locked && db.Dialector.Name() == "mysql" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.Find(rows).Error; err != nil {
			return nil, err
		}
	}
	q := db.WithContext(ctx).Table("users").Select("id, status, deleted_at").Order("id ASC")
	if locked && db.Dialector.Name() == "mysql" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Find(&s.Users).Error; err != nil {
		return nil, err
	}
	return s, nil
}
func AnalyzeAuthzHardening(ctx context.Context, db *gorm.DB, providers *objectattributeadmission.Registry) (*AuthzHardeningReport, error) {
	if db == nil {
		return nil, fmt.Errorf("authorization database required")
	}
	var report *AuthzHardeningReport
	read := func(tx *gorm.DB) error {
		state, err := loadHardeningState(ctx, tx, false, providers)
		if err != nil {
			return err
		}
		report, err = analyzeHardeningState(ctx, state, providers)
		return err
	}
	var err error
	if db.Dialector.Name() == "mysql" {
		err = db.WithContext(ctx).Transaction(read, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	} else {
		err = db.WithContext(ctx).Transaction(read)
	}
	return report, err
}
func analyzeHardeningState(ctx context.Context, s *hardeningState, providers *objectattributeadmission.Registry) (*AuthzHardeningReport, error) {
	encoded, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	report := &AuthzHardeningReport{Fingerprint: fmt.Sprintf("%x", sha256.Sum256(encoded)), ProviderFingerprint: s.ProviderFingerprint, Blockers: []HardeningIssue{}, Changes: []HardeningChange{}, PlatformWriters: []string{}, PolicyVersions: map[string]int64{}}
	issue := func(kind, object, detail string) {
		report.Blockers = append(report.Blockers, HardeningIssue{kind, object, detail})
	}
	data := authzruntime.Dataset{Versions: report.PolicyVersions}
	roles := map[meta.ID]string{}
	nodes := []roleinheritance.RoleNode{}
	resources := map[uint64]*resource.Resource{}
	users := map[string]bool{}
	existingUsers := map[string]bool{}
	for _, u := range s.Users {
		users[u.ID.String()] = u.DeletedAt == nil && u.Status == 1
		existingUsers[u.ID.String()] = u.DeletedAt == nil
	}
	for _, r := range s.Roles {
		if r.DeletedAt != nil {
			continue
		}
		roles[r.ID] = r.TenantID
		nodes = append(nodes, roleinheritance.RoleNode{ID: r.ID, TenantID: r.TenantID})
		data.Roles = append(data.Roles, authzruntime.RoleRecord{ID: r.ID, TenantID: r.TenantID, Name: r.Name})
	}
	for _, r := range s.Resources {
		if r.DeletedAt != nil {
			continue
		}
		bo, err := resourcerepo.NewMapper().ToBO(r)
		if err != nil {
			issue("invalid_resource", r.ID.String(), fmt.Sprintf("%+v", err))
			continue
		}
		resources[r.ID.Uint64()] = bo
		data.Resources = append(data.Resources, bo)
	}
	for _, a := range s.Assignments {
		if a.DeletedAt != nil {
			continue
		}
		if roles[meta.ID(a.RoleID)] != a.TenantID {
			issue("invalid_assignment_role", a.ID.String(), fmt.Sprintf("tenant %s role %d", a.TenantID, a.RoleID))
		}
		if a.SubjectType == "user" && !existingUsers[a.SubjectID] {
			issue("unavailable_assignment_subject", a.ID.String(), a.SubjectType+":"+a.SubjectID)
		}
		if _, err := subject.ParseRef(a.SubjectType + ":" + a.SubjectID); err != nil {
			issue("invalid_subject", a.ID.String(), a.SubjectType+":"+a.SubjectID)
		}
		data.Assignments = append(data.Assignments, authzruntime.AssignmentRecord{TenantID: a.TenantID, SubjectKey: a.SubjectType + ":" + a.SubjectID, RoleID: meta.ID(a.RoleID)})
	}
	edges := []*roleinheritance.Inheritance{}
	for _, e := range s.Inheritances {
		if e.DeletedAt != nil || e.RevokedAt != nil {
			continue
		}
		if roles[meta.ID(e.RoleID)] != e.TenantID || roles[meta.ID(e.InheritedRoleID)] != e.TenantID {
			issue("invalid_inheritance_reference", e.ID.String(), fmt.Sprintf("tenant %s roles %d -> %d", e.TenantID, e.RoleID, e.InheritedRoleID))
		}
		edges = append(edges, &roleinheritance.Inheritance{ID: e.ID, TenantID: tenant.ID(e.TenantID), RoleID: meta.ID(e.RoleID), InheritedRoleID: meta.ID(e.InheritedRoleID)})
		data.Inheritances = append(data.Inheritances, authzruntime.InheritanceRecord{TenantID: e.TenantID, RoleID: meta.ID(e.RoleID), InheritedRoleID: meta.ID(e.InheritedRoleID)})
	}
	if err := roleinheritance.ValidateGraph(nodes, edges); err != nil {
		issue("invalid_inheritance_graph", "role_inheritances", fmt.Sprintf("%+v", err))
	}
	for _, g := range s.Grants {
		if g.DeletedAt != nil || g.RevokedAt != nil {
			continue
		}
		bo, err := (grantrepo.Mapper{}).ToBO(g)
		if err != nil {
			issue("invalid_grant", g.ID.String(), fmt.Sprintf("%+v", err))
			continue
		}
		if roles[bo.RoleID] != g.TenantID {
			issue("invalid_grant_role", g.ID.String(), fmt.Sprintf("tenant %s role %d", g.TenantID, g.RoleID))
		}
		if g.ResourceID != nil {
			r := resources[*g.ResourceID]
			if r == nil {
				issue("invalid_grant_resource", g.ID.String(), fmt.Sprint(*g.ResourceID))
			} else if err := bo.ValidateAgainst(*r); err != nil {
				issue("unsatisfiable_grant", g.ID.String(), fmt.Sprintf("%+v", err))
			}
		}
		if err := providers.ValidateCoverage(bo.ResourcePatternString(), bo.Constraints); err != nil {
			issue("missing_attribute_provider", g.ID.String(), err.Error())
		}
		data.Grants = append(data.Grants, bo)
		if g.TenantID != "platform" && g.ResourcePattern == authorizationapp.ResourceResources {
			switch g.Action {
			case "create", "update", "delete":
				report.Changes = append(report.Changes, HardeningChange{Grant: bo})
			case "*":
				report.Changes = append(report.Changes, HardeningChange{Grant: bo, ReplacementActions: []string{"read", "list", "validate_action"}})
			}
		}
	}
	for _, v := range s.Versions {
		if v.DeletedAt == nil && v.PolicyVersion > data.Versions[v.TenantID] {
			data.Versions[v.TenantID] = v.PolicyVersion
		}
	}
	if len(report.Blockers) == 0 {
		runtime, err := authzruntime.NewRuntime(ctx, hardeningSource{data}, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(providers))
		if err != nil {
			issue("invalid_snapshot", "runtime", fmt.Sprintf("%+v", err))
		} else {
			seen := map[string]bool{}
			for _, a := range data.Assignments {
				if a.TenantID != "platform" || seen[a.SubjectKey] {
					continue
				}
				seen[a.SubjectKey] = true
				sub, err := subject.ParseRef(a.SubjectKey)
				if err != nil || sub.Type != subject.TypeUser || !users[sub.ID.String()] {
					continue
				}
				allowed := true
				for _, action := range []string{"create", "update", "delete"} {
					request, err := authorization.NewRequest(sub, "platform", authorizationapp.ResourceResources, action, authorization.ObjectContext{})
					if err != nil {
						allowed = false
						break
					}
					decision, err := runtime.Check(ctx, request)
					if err != nil || !decision.Allowed {
						allowed = false
						break
					}
				}
				if allowed {
					report.PlatformWriters = append(report.PlatformWriters, a.SubjectKey)
				}
			}
		}
	}
	if len(report.PlatformWriters) == 0 {
		if len(report.Blockers) > 0 {
			issue("platform_catalog_writer_unverified", "platform", "resolve invalid facts before proving an existing platform writer")
		} else {
			issue("missing_platform_catalog_writer", "platform", "no existing active user can create/update/delete the catalog")
		}
	}
	sort.Strings(report.PlatformWriters)
	report.Complete = len(report.Blockers) == 0 && len(report.Changes) == 0
	return report, nil
}

type hardeningSource struct{ data authzruntime.Dataset }

func (s hardeningSource) Load(context.Context) (authzruntime.Dataset, error) { return s.data, nil }

func ApplyAuthzHardening(ctx context.Context, db *gorm.DB, uow authzuow.UnitOfWork, providers *objectattributeadmission.Registry, expectedFingerprint string) (*AuthzHardeningReport, error) {
	if db == nil || uow == nil || len(strings.TrimSpace(expectedFingerprint)) != sha256.Size*2 {
		return nil, fmt.Errorf("database, transaction and expected fingerprint required")
	}
	var report *AuthzHardeningReport
	options := dbmysql.TxOptions{Name: "authz-hardening"}
	if db.Dialector.Name() == "mysql" {
		options.Isolation = sql.LevelRepeatableRead
	}
	err := dbmysql.NewUnitOfWork(db).WithinTransaction(ctx, func(ctx context.Context) error {
		return uow.WithinTx(ctx, func(txCtx context.Context, repos authzuow.TxRepositories) error {
			tx, err := dbmysql.RequireTx(txCtx)
			if err != nil {
				return err
			}
			state, err := loadHardeningState(txCtx, tx, true, providers)
			if err != nil {
				return err
			}
			report, err = analyzeHardeningState(txCtx, state, providers)
			if err != nil {
				return err
			}
			if report.Fingerprint != expectedFingerprint {
				return fmt.Errorf("approved data fingerprint changed")
			}
			if len(report.Blockers) > 0 {
				return fmt.Errorf("authorization hardening blocked")
			}
			if len(report.Changes) == 0 {
				return nil
			}
			before := report
			tenants := map[string]bool{}
			for _, change := range before.Changes {
				old := change.Grant
				catalogResource, err := repos.Resources.FindByKey(txCtx, authorizationapp.ResourceResources)
				if err != nil {
					return err
				}
				for _, action := range change.ReplacementActions {
					replacement, err := permissiongrant.New(old.RoleID, old.TenantIDString(), catalogResource.ID, old.ResourcePatternString(), action, old.Constraints, hardeningActor)
					if err != nil {
						return err
					}
					existing, err := repos.PermissionGrants.ListByRole(txCtx, old.RoleID, old.TenantIDString())
					if err != nil {
						return err
					}
					found := false
					for _, g := range existing {
						if g.IsActive() && g.GrantKey == replacement.GrantKey {
							found = true
							break
						}
					}
					if !found {
						if err := repos.PermissionGrants.Create(txCtx, &replacement); err != nil {
							return err
						}
					}
				}
				outcome, err := repos.PermissionGrants.AtomicRevoke(txCtx, old.ID, old.TenantIDString())
				if err != nil {
					return err
				}
				if outcome != permissiongrant.RevokeOutcomeRevoked {
					return fmt.Errorf("reviewed grant changed during hardening")
				}
				tenants[old.TenantIDString()] = true
			}
			ordered := make([]string, 0, len(tenants))
			for id := range tenants {
				ordered = append(ordered, id)
			}
			sort.Strings(ordered)
			for _, id := range ordered {
				version, err := repos.PolicyVersions.Increment(txCtx, id, hardeningActor, "platform catalog write hardening")
				if err != nil {
					return err
				}
				if err := policychange.StagePolicyVersionChanged(txCtx, repos.Events, id, version); err != nil {
					return err
				}
			}
			state, err = loadHardeningState(txCtx, tx, false, providers)
			if err != nil {
				return err
			}
			report, err = analyzeHardeningState(txCtx, state, providers)
			if err != nil {
				return err
			}
			if !report.Complete {
				return fmt.Errorf("hardening postconditions failed")
			}
			report.BeforeFingerprint = before.Fingerprint
			report.AppliedChanges = before.Changes
			return nil
		})
	}, options)
	return report, err
}
