package rolemodel

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	mysqlseed "github.com/FangcunMount/iam/v5/configs/mysql"
	legacygrant "github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/legacycondition/grant"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"

	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/gorm"
)

// PrepareBootstrapTenant is exclusively a fresh-database migration hook. The
// migrator proves the database is empty before running any historical SQL.
// Existing databases must use the reviewed offline upgrade workflow.
func PrepareBootstrapTenant(ctx context.Context, db *gorm.DB) error {
	db = db.WithContext(ctx)
	// MySQL commits an active transaction around CREATE TABLE, even when the
	// table already exists. Prepare it before changing authorization data.
	if err := db.Exec("CREATE TABLE IF NOT EXISTS iam_authorization_retirement_preparation (id TINYINT UNSIGNED PRIMARY KEY,source_hash VARCHAR(64) NOT NULL,prepared_hash VARCHAR(64) NOT NULL,initial_version BIGINT NOT NULL,prepared_at DATETIME(3) NOT NULL) ENGINE=InnoDB").Error; err != nil {
		return err
	}
	var engine string
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_authorization_retirement_preparation'").Scan(&engine).Error; err != nil {
		return err
	}
	if !strings.EqualFold(engine, "InnoDB") {
		return fmt.Errorf("fresh bootstrap preparation receipt requires InnoDB")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var unexpected int64
		if err := tx.Raw("SELECT COUNT(*) FROM authz_assignments a JOIN authz_roles r ON r.id=a.role_id WHERE r.name IN ('platform:admin','iam:admin') AND a.deleted_at IS NULL").Scan(&unexpected).Error; err != nil {
			return err
		}
		if unexpected != 0 {
			return fmt.Errorf("historical bootstrap aliases have assignments")
		}
		for _, query := range []string{
			"UPDATE authz_permission_grants g JOIN authz_roles r ON r.id=g.role_id SET g.revoked_at=NOW(),g.deleted_at=NOW() WHERE r.name IN ('platform:admin','iam:admin')",
			"UPDATE authz_roles SET deleted_at=NOW() WHERE name IN ('platform:admin','iam:admin')",
			"UPDATE authz_roles SET name='platform_admin' WHERE name='super_admin' AND tenant_id='platform'",
			"UPDATE authz_roles SET name='iam_admin' WHERE name='tenant_admin' AND tenant_id='fangcun'",
			"UPDATE authz_resources SET actions=JSON_ARRAY_APPEND(actions,'$','list_all','$','search_by_mobile_all') WHERE `key`='iam:identity:collection:profiles' AND JSON_CONTAINS(actions,JSON_QUOTE('list_all'))=0",
			"UPDATE authz_resources SET actions=JSON_ARRAY_APPEND(actions,'$','manage_protected') WHERE `key`='iam:authz:collection:roles' AND JSON_CONTAINS(actions,JSON_QUOTE('manage_protected'))=0",
		} {
			if err := tx.Exec(query).Error; err != nil {
				return err
			}
		}
		var grants []grantpo.GrantPO
		if err := tx.Unscoped().Order("id").Find(&grants).Error; err != nil {
			return err
		}
		for _, row := range grants {
			grant, err := (legacygrant.Mapper{}).ToBO(&row)
			if err != nil {
				return err
			}
			if err = tx.Table("authz_permission_grants").Where("id = ?", row.ID).Update("grant_key", grant.GrantKey).Error; err != nil {
				return err
			}
		}
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(encode(grants))))
		return tx.Exec("INSERT INTO iam_authorization_retirement_preparation(id,source_hash,prepared_hash,initial_version,prepared_at) SELECT 1,?,?,COALESCE(MAX(policy_version),0)+1,NOW(3) FROM authz_policy_versions", digest, digest).Error
	})
}

func BootstrapIndependentRoles(ctx context.Context, db *gorm.DB, stager event.Stager) error {
	var fingerprint string
	err := dbmysql.NewUnitOfWork(db).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := dbmysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		if err := lockPolicy(tx); err != nil {
			return err
		}
		// The original policy changes, notification and receipt commit together.
		// A retry can finish the separately verified archive from that receipt.
		var applied Receipt
		existing := tx.Select("fingerprint").Where("migration_id = ?", MigrationID).Limit(1).Find(&applied)
		if existing.Error != nil {
			return existing.Error
		}
		if existing.RowsAffected != 0 {
			fingerprint = applied.Fingerprint
			return nil
		}
		before, err := LoadState(txCtx, tx)
		if err != nil {
			return err
		}
		// These are unassigned historical seed identities, never production people.
		for _, name := range []string{"qs:staff", "qs:evaluator", "super_admin"} {
			var n int64
			if err = tx.Raw("SELECT COUNT(*) FROM authz_assignments a JOIN authz_roles r ON r.id=a.role_id WHERE r.name=? AND a.deleted_at IS NULL", name).Scan(&n).Error; err != nil {
				return err
			}
			if n != 0 {
				return fmt.Errorf("fresh bootstrap role unexpectedly assigned: %s", name)
			}
			if err = tx.Exec("UPDATE authz_roles SET deleted_at=NOW() WHERE name=?", name).Error; err != nil {
				return err
			}
		}
		// The embedded baseline terminates each statement on its own line.
		// Execute statements individually; business connections need no multiStatements.
		for _, statement := range strings.Split(mysqlseed.SQL, ";\n") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if err = tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		if err = advance(txCtx, tx, stager, "independent role bootstrap"); err != nil {
			return err
		}
		after, err := LoadState(txCtx, tx)
		if err != nil {
			return err
		}
		in, err := after.Input(nil)
		if err != nil {
			return err
		}
		p := Plan{MigrationID: MigrationID, Ready: true, Grants: map[string][]Permission{}}
		for _, r := range in.Roles {
			p.Grants[r.Name] = []Permission{}
		}
		for _, g := range in.Grants {
			p.Grants[g.Role] = append(p.Grants[g.Role], g.Permission)
		}
		people := map[string][]string{}
		for _, a := range in.Assignments {
			people[a.Subject] = append(people[a.Subject], a.Role)
		}
		for sub, roles := range people {
			p.People = append(p.People, Mapping{Subject: sub, Before: roles, After: roles})
		}
		if err = verifyPlan(after, p); err != nil {
			return err
		}
		fingerprint = before.Hash()
		p.Fingerprint = fingerprint
		return tx.Create(&Receipt{MigrationID: MigrationID, Fingerprint: fingerprint, AfterHash: after.Hash(), Status: "applied", BeforeJSON: encode(before), AfterJSON: encode(after), PlanJSON: encode(p), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}).Error
	})
	if err != nil {
		return err
	}
	_, err = ArchiveInheritance(ctx, db, fingerprint, true)
	return err
}
