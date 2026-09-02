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
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
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
func (l *Loader) Full(ctx context.Context) ([]domainprofile.SuggestibleProfile, error) {
	rows, err := l.queryRows(ctx, l.config.FullSQL)
	if err != nil {
		return nil, err
	}
	out := make([]domainprofile.SuggestibleProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.fullProfile())
	}
	log.Infow("suggest loader finished query", "count", len(out))
	return out, nil
}

func (l *Loader) queryRows(ctx context.Context, sql string, args ...interface{}) ([]record, error) {
	if l.db == nil {
		return nil, fmt.Errorf("suggest loader db is nil")
	}

	var rows []record
	if err := l.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Delta 增量拉取，按时间过滤；在 adapter 边界将空 name 转为显式 Delete。
func (l *Loader) Delta(ctx context.Context, since time.Time) ([]apprefresh.ProjectionChange, error) {
	if strings.TrimSpace(l.config.DeltaSQL) == "" {
		return nil, nil
	}
	var rows []record
	var err error
	if l.defaultDelta {
		rows, err = l.queryRows(ctx, l.config.DeltaSQL, since, since, since, since, since, since, since)
	} else {
		rows, err = l.queryRows(ctx, l.config.DeltaSQL, since, since)
	}
	if err != nil {
		return nil, err
	}
	out := make([]apprefresh.ProjectionChange, 0, len(rows))
	for _, row := range rows {
		ch, err := row.deltaChange()
		if err != nil {
			continue
		}
		out = append(out, ch)
	}
	return out, nil
}

type record struct {
	ID               int64   `gorm:"column:id"`
	Name             string  `gorm:"column:name"`
	OrgID            int64   `gorm:"column:org_id"`
	Mobiles          *string `gorm:"column:mobiles"`
	OwnerOperatorIDs *string `gorm:"column:owner_operator_ids"`
	Weight           int     `gorm:"column:weight"`
}

func (r record) fullProfile() domainprofile.SuggestibleProfile {
	mobiles := ""
	if r.Mobiles != nil {
		mobiles = *r.Mobiles
	}
	owners := ""
	if r.OwnerOperatorIDs != nil {
		owners = *r.OwnerOperatorIDs
	}
	return domainprofile.RawProjection(
		r.ID,
		r.Name,
		splitMobiles(mobiles),
		r.Weight,
		r.OrgID,
		splitInt64CSV(owners),
	)
}

func (r record) deltaChange() (apprefresh.ProjectionChange, error) {
	if r.ID <= 0 {
		return apprefresh.ProjectionChange{}, fmt.Errorf("profile id required for delta change")
	}
	if strings.TrimSpace(r.Name) == "" {
		return apprefresh.Delete(r.ID)
	}
	mobiles := ""
	if r.Mobiles != nil {
		mobiles = *r.Mobiles
	}
	owners := ""
	if r.OwnerOperatorIDs != nil {
		owners = *r.OwnerOperatorIDs
	}
	p, err := domainprofile.New(
		r.ID,
		r.Name,
		splitMobiles(mobiles),
		r.Weight,
		r.OrgID,
		splitInt64CSV(owners),
	)
	if err != nil {
		return apprefresh.ProjectionChange{}, err
	}
	return apprefresh.Upsert(p)
}

var _ apprefresh.ProjectionSource = (*Loader)(nil)

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
