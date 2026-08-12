// Package suggest 从 MySQL 加载档案联想索引项。
// 默认 SQL 为过渡读模型：org_id 来自 PlaceholderOrgID（profiles 表尚无 org 列），
// 授权域由 JWT tenant domain 承担，不进入索引；owner_operator_ids 来自 profiles.created_by。
package suggest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	"gorm.io/gorm"
)

const (
	defaultFullSQLTemplate = `
SELECT
  c.id,
  c.name,
  %d AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL AND g.revoked_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id;
`
	// 受影响档案重新计算：仍有活跃 User/ProfileLink 时 upsert，否则 tombstone。
	defaultDeltaSQLTemplate = `
WITH affected_profiles AS (
  SELECT c.id AS profile_id
  FROM profiles c
  WHERE c.updated_at > ? OR c.deleted_at > ?
  UNION
  SELECT g.profile_id
  FROM profile_links g
  WHERE g.updated_at > ? OR g.deleted_at > ? OR g.revoked_at > ?
  UNION
  SELECT g.profile_id
  FROM profile_links g
  INNER JOIN users u ON u.id = g.user_id
  WHERE u.updated_at > ? OR u.deleted_at > ?
),
eligible_profiles AS (
SELECT
  c.id,
  c.name,
  %d AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN affected_profiles a ON a.profile_id = c.id
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL AND g.revoked_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id, c.name, c.created_by
)
SELECT id, name, org_id, mobiles, owner_operator_ids, weight
FROM eligible_profiles
UNION ALL
SELECT
  c.id,
  '' AS name,
  %d AS org_id,
  '' AS mobiles,
  CAST(c.created_by AS CHAR) AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN affected_profiles a ON a.profile_id = c.id
WHERE NOT EXISTS (
  SELECT 1 FROM eligible_profiles e WHERE e.id = c.id
);
`
)

// LoaderConfig 提供 SQL 可配置能力。
// PlaceholderOrgID：当 profiles 尚无 org_id 列时，内建 SQL 注入的占位业务组织 ID。
// 0 表示不在索引中虚构 org；单组织部署由业务配置占位值，或改用 FullSQL。
type LoaderConfig struct {
	FullSQL          string
	DeltaSQL         string
	PlaceholderOrgID int64
}

// Loader 从业务库拉取档案联想候选
type Loader struct {
	db           *gorm.DB
	config       LoaderConfig
	defaultDelta bool
}

// NewLoader 创建 Loader，SQL 为空时使用默认值。
func NewLoader(db *gorm.DB, cfg LoaderConfig) *Loader {
	placeholderOrg := cfg.PlaceholderOrgID
	fullSQL := strings.TrimSpace(cfg.FullSQL)
	if fullSQL == "" {
		fullSQL = strings.TrimSpace(fmt.Sprintf(defaultFullSQLTemplate, placeholderOrg))
	}
	deltaSQL := strings.TrimSpace(cfg.DeltaSQL)
	defaultDelta := deltaSQL == ""
	if defaultDelta {
		deltaSQL = strings.TrimSpace(fmt.Sprintf(defaultDeltaSQLTemplate, placeholderOrg, placeholderOrg))
	}

	return &Loader{
		db: db,
		config: LoaderConfig{
			FullSQL:          fullSQL,
			DeltaSQL:         deltaSQL,
			PlaceholderOrgID: placeholderOrg,
		},
		defaultDelta: defaultDelta,
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
	if l.defaultDelta {
		return l.query(ctx, l.config.DeltaSQL, since, since, since, since, since, since, since)
	}
	return l.query(ctx, l.config.DeltaSQL, since, since)
}

type record struct {
	ID               int64   `gorm:"column:id"`
	Name             string  `gorm:"column:name"`
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
