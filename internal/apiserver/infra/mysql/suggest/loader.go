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
  %d AS tenant_id,
  0 AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  '' AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id;
`
	defaultDeltaSQLTemplate = `
SELECT
  c.id,
  c.name,
  %d AS tenant_id,
  0 AS org_id,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  '' AS owner_operator_ids,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL AND GREATEST(c.updated_at, g.updated_at, u.updated_at) > ?
GROUP BY c.id;
`
)

// LoaderConfig 提供 SQL 可配置能力。
// PlaceholderTenantID：当 profiles 尚无真实 tenant_id 列时，内建 SQL 使用的占位租户 ID。
// 0 表示不在索引中虚构租户——此时 tenant 维度的 scope 与索引需一致才命中；单租户开发可配置为非 0（与 Principal.TenantID 对齐），或改用 FullSQL 读取真实列。
type LoaderConfig struct {
	FullSQL             string
	DeltaSQL            string
	PlaceholderTenantID int64
}

// Loader 从业务库拉取档案联想候选
type Loader struct {
	db     *gorm.DB
	config LoaderConfig
}

// NewLoader 创建 Loader，SQL 为空时使用默认值。
func NewLoader(db *gorm.DB, cfg LoaderConfig) *Loader {
	fullSQL := strings.TrimSpace(cfg.FullSQL)
	if fullSQL == "" {
		fullSQL = strings.TrimSpace(fmt.Sprintf(defaultFullSQLTemplate, cfg.PlaceholderTenantID))
	}
	deltaSQL := strings.TrimSpace(cfg.DeltaSQL)
	if deltaSQL == "" {
		deltaSQL = strings.TrimSpace(fmt.Sprintf(defaultDeltaSQLTemplate, cfg.PlaceholderTenantID))
	}

	return &Loader{
		db: db,
		config: LoaderConfig{
			FullSQL:             fullSQL,
			DeltaSQL:            deltaSQL,
			PlaceholderTenantID: cfg.PlaceholderTenantID,
		},
	}
}

// Full 全量拉取
func (l *Loader) Full(ctx context.Context) ([]domainsuggest.ProfileSearchTerm, error) {
	return l.query(ctx, l.config.FullSQL)
}

// Delta 增量拉取，按时间过滤。
// 当前索引实现仅追加键、不删除旧键；生产建议 delta_sync_cron 留空，仅全量刷新。
func (l *Loader) Delta(ctx context.Context, since time.Time) ([]domainsuggest.ProfileSearchTerm, error) {
	if strings.TrimSpace(l.config.DeltaSQL) == "" {
		return nil, nil
	}
	return l.query(ctx, l.config.DeltaSQL, since)
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

	log.Infow("suggest loader finished query", "sql", sanitizeSQL(sql), "count", len(out))

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

// sanitizeSQL 仅用于日志，避免输出换行
func sanitizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
