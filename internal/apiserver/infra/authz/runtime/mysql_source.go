package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	assignmentrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	permissiongrantrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
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
	if err := db.Where("deleted_at IS NULL").Order("id ASC").Find(&roleRows).Error; err != nil {
		return Dataset{}, err
	}
	roles := make([]RoleRecord, 0, len(roleRows))
	for _, row := range roleRows {
		roles = append(roles, RoleRecord{ID: row.ID, Name: row.Name, ManagementProtection: role.ManagementProtection(row.ManagementProtection)})
	}

	var assignmentRows []*assignmentrepo.AssignmentPO
	if err := db.Where("deleted_at IS NULL").Order("id ASC").Find(&assignmentRows).Error; err != nil {
		return Dataset{}, err
	}
	assignments := make([]AssignmentRecord, 0, len(assignmentRows))
	for _, row := range assignmentRows {
		mapped, err := assignmentrepo.NewMapper().ToBO(row)
		if err != nil {
			return Dataset{}, err
		}
		record := AssignmentRecord{ID: meta.ID(mapped.ID), SubjectKey: row.SubjectType + ":" + row.SubjectID, RoleID: mapped.RoleID}
		if value, ok := mapped.Scope(); ok {
			record.Scope = &value
		}
		assignments = append(assignments, record)
	}

	var grantRows []*permissiongrantrepo.GrantPO
	if err := db.Where("deleted_at IS NULL AND revoked_at IS NULL").Order("id ASC").Find(&grantRows).Error; err != nil {
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

	version, err := readVersion(db)
	if err != nil {
		return Dataset{}, err
	}

	return Dataset{
		Roles: roles, Assignments: assignments,
		Grants: grants, Resources: resources, Version: version,
	}, nil
}

func (s *MySQLSource) ReadVersion(ctx context.Context) (int64, error) {
	return readVersion(s.db.WithContext(ctx))
}
func readVersion(db *gorm.DB) (int64, error) {
	var row policyrepo.PolicyVersionPO
	if err := db.Where("id = ?", 1).First(&row).Error; err != nil {
		return 0, err
	}
	return row.PolicyVersion, nil
}
