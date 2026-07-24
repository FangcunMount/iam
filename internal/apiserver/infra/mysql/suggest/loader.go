// Package suggest 从 MySQL 加载档案联想索引项。
// 默认 SQL 为过渡读模型：org_id 来自 PlaceholderOrgID（profiles 表尚无 org 列），
// tenant_id 固定为 0（授权域由 JWT tenant domain 承担，不入索引），owner_operator_ids 来自 profiles.created_by。
package suggest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"gorm.io/gorm"
)

const (
	defaultFullSQLTemplate = `
SELECT
  c.id,
  c.name,
  0 AS tenant_id,
  %d AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id;
`
	// 活跃档案 upsert + 软删除 tombstone（name 为空，索引层会 RemoveProfile）。
	defaultDeltaSQLTemplate = `
SELECT
  c.id,
  c.name,
  0 AS tenant_id,
  %d AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL AND GREATEST(c.updated_at, g.updated_at, u.updated_at) > ?
GROUP BY c.id
UNION ALL
SELECT
  c.id,
  '' AS name,
  0 AS tenant_id,
  %d AS org_id,
  '' AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
WHERE c.deleted_at IS NOT NULL AND c.deleted_at > ?;
`
)

// LoaderConfig 提供 SQL 可配置能力。
// PlaceholderOrgID：当 profiles 尚无 org_id 列时，内建 SQL 注入的占位业务组织 ID。
// 0 表示不在索引中虚构 org；单组织部署由业务配置占位值，或改用 FullSQL。
type LoaderConfig struct {
	FullSQL             string
	DeltaSQL            string
	PlaceholderOrgID    int64
	PlaceholderTenantID int64 // Deprecated: 与 PlaceholderOrgID 同义，仅配置兼容。
}

// Loader 从业务库拉取档案联想候选
type Loader struct {
	db     *gorm.DB
	config LoaderConfig
}

// NewLoader 创建 Loader，SQL 为空时使用默认值。
func NewLoader(db *gorm.DB, cfg LoaderConfig) *Loader {
	placeholderOrg := cfg.PlaceholderOrgID
	if placeholderOrg == 0 {
		placeholderOrg = cfg.PlaceholderTenantID
	}
	fullSQL := strings.TrimSpace(cfg.FullSQL)
	if fullSQL == "" {
		fullSQL = strings.TrimSpace(fmt.Sprintf(defaultFullSQLTemplate, placeholderOrg))
	}
	deltaSQL := strings.TrimSpace(cfg.DeltaSQL)
	if deltaSQL == "" {
		deltaSQL = strings.TrimSpace(fmt.Sprintf(defaultDeltaSQLTemplate, placeholderOrg, placeholderOrg))
	}

	return &Loader{
		db: db,
		config: LoaderConfig{
			FullSQL:             fullSQL,
			DeltaSQL:            deltaSQL,
			PlaceholderOrgID:    placeholderOrg,
			PlaceholderTenantID: cfg.PlaceholderTenantID,
		},
	}
}

// Full 全量拉取
func (l *Loader) Full(ctx context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return l.query(ctx, l.config.FullSQL)
}

// Delta 增量拉取，按时间过滤。
// 索引层：同一 profileID 会先撤销旧 Trie/Hash 键；DisplayName 为空视为删除。
// 默认 Delta SQL：返回 since 之后更新的活跃档案，以及 since 之后软删除的 tombstone（name=”）。
// 自定义 DeltaSQL 须自行保证 tombstone 协议，或仅全量刷新。
func (l *Loader) Delta(ctx context.Context, since time.Time) ([]domainsuggest.ProfileSearchTerm, error) {
	if strings.TrimSpace(l.config.DeltaSQL) == "" {
		return nil, nil
	}
	return l.query(ctx, l.config.DeltaSQL, since, since)
}

type record struct {
	ID               int64   `gorm:"column:id"`
	Name             string  `gorm:"column:name"`
	TenantID         int64   `gorm:"column:tenant_id"`
	OrgID            int64   `gorm:"column:org_id"`
	Mobiles          *string `gorm:"column:mobiles"`
	OwnerOperatorIDs *string `gorm:"column:owner_operator_ids"`
	Weight           int     `gorm:"column:weight"`
}

func (l *Loader) query(ctx context.Context, sql string, args ...interface{}) ([]domainsuggest.ProfileSearchTerm, error) {
	if l.db == nil {
		return nil, fmt.Errorf("suggest loader db is nil")
	}

	var rows []record
	if err := l.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]domainsuggest.ProfileSearchTerm, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.profileSearchTerm())
	}

	log.Infow("suggest loader finished query", "count", len(out))

	return out, nil
}

func (r record) profileSearchTerm() domainsuggest.ProfileSearchTerm {
	mobiles := ""
	if r.Mobiles != nil {
		mobiles = *r.Mobiles
	}
	owners := ""
	if r.OwnerOperatorIDs != nil {
		owners = *r.OwnerOperatorIDs
	}
	return domainsuggest.NewProfileSearchTerm(
		r.ID,
		r.Name,
		splitMobiles(mobiles),
		r.Weight,
		r.TenantID,
		r.OrgID,
		splitInt64CSV(owners),
	)
}

func splitInt64CSV(value string) []int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func splitMobiles(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	mobiles := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mobiles = append(mobiles, part)
	}
	return mobiles
}
