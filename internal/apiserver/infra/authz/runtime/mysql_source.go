package runtime

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	permissiongrantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	roleinheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
)

type MySQLSource struct {
	db *gorm.DB
}

func NewMySQLSource(db *gorm.DB) *MySQLSource { return &MySQLSource{db: db} }

func (s *MySQLSource) Load(ctx context.Context) (Dataset, error) {
	if s == nil || s.db == nil {
		return Dataset{}, fmt.Errorf("authorization runtime database is required")
	}
	var dataset Dataset
	load := func(tx *gorm.DB) error {
		var err error
		dataset, err = loadDataset(tx.WithContext(ctx))
		return err
	}
	if s.db.Dialector != nil && s.db.Dialector.Name() == "mysql" {
		if err := s.db.WithContext(ctx).Transaction(load, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}); err != nil {
			return Dataset{}, err
		}
	} else if err := s.db.WithContext(ctx).Transaction(load); err != nil {
		return Dataset{}, err
	}
	return dataset, nil
}

func loadDataset(db *gorm.DB) (Dataset, error) {
	var roleRows []*rolerepo.RolePO
	if err := db.Where("deleted_at IS NULL").Order("tenant_id ASC, id ASC").Find(&roleRows).Error; err != nil {
		return Dataset{}, err
	}
	roles := make([]RoleRecord, 0, len(roleRows))
	for _, row := range roleRows {
		roles = append(roles, RoleRecord{ID: row.ID, TenantID: row.TenantID, Name: row.Name})
	}

	var assignmentRows []*assignmentrepo.AssignmentPO
	if err := db.Where("deleted_at IS NULL").Order("tenant_id ASC, id ASC").Find(&assignmentRows).Error; err != nil {
		return Dataset{}, err
	}
	assignments := make([]AssignmentRecord, 0, len(assignmentRows))
	for _, row := range assignmentRows {
		assignments = append(assignments, AssignmentRecord{
			TenantID: row.TenantID, SubjectKey: row.SubjectType + ":" + row.SubjectID,
			RoleID: meta.FromUint64(row.RoleID),
		})
	}

	var inheritanceRows []*roleinheritancerepo.InheritancePO
	if err := db.Where("deleted_at IS NULL AND revoked_at IS NULL").Order("tenant_id ASC, id ASC").Find(&inheritanceRows).Error; err != nil {
		return Dataset{}, err
	}
	inheritances := make([]InheritanceRecord, 0, len(inheritanceRows))
	for _, row := range inheritanceRows {
		inheritances = append(inheritances, InheritanceRecord{
			TenantID: row.TenantID, RoleID: meta.FromUint64(row.RoleID),
			InheritedRoleID: meta.FromUint64(row.InheritedRoleID),
		})
	}

	var grantRows []*permissiongrantrepo.GrantPO
	if err := db.Where("deleted_at IS NULL AND revoked_at IS NULL").Order("tenant_id ASC, id ASC").Find(&grantRows).Error; err != nil {
		return Dataset{}, err
	}
	grantMapper := permissiongrantrepo.Mapper{}
	grants := make([]*permissiongrant.Grant, 0, len(grantRows))
	for _, row := range grantRows {
		grant, err := grantMapper.ToBO(row)
		if err != nil {
			return Dataset{}, err
		}
		grants = append(grants, grant)
	}

	var resourceRows []*resourcerepo.ResourcePO
	if err := db.Where("deleted_at IS NULL").Order("`key` ASC").Find(&resourceRows).Error; err != nil {
		return Dataset{}, err
	}
	resourceMapper := resourcerepo.NewMapper()
	resources := make([]*resource.Resource, 0, len(resourceRows))
	for _, row := range resourceRows {
		catalogResource, err := resourceMapper.ToBO(row)
		if err != nil {
			return Dataset{}, err
		}
		resources = append(resources, catalogResource)
	}

	type versionRow struct {
		TenantID string `gorm:"column:tenant_id"`
		Version  int64  `gorm:"column:policy_version"`
	}
	var versionRows []versionRow
	if err := db.Model(&policyrepo.PolicyVersionPO{}).
		Select("tenant_id, MAX(policy_version) AS policy_version").
		Where("deleted_at IS NULL").Group("tenant_id").Scan(&versionRows).Error; err != nil {
		return Dataset{}, err
	}
	versions := make(map[string]int64, len(versionRows))
	for _, row := range versionRows {
		versions[row.TenantID] = row.Version
	}

	return Dataset{
		Roles: roles, Assignments: assignments, Inheritances: inheritances,
		Grants: grants, Resources: resources, Versions: versions,
	}, nil
}
