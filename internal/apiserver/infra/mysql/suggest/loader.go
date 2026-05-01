package suggest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"gorm.io/gorm"
)

const (
	defaultFullSQL = `
SELECT
  c.id,
  c.name,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id;
`
	defaultDeltaSQL = `
SELECT
  c.id,
  c.name,
  GROUP_CONCAT(DISTINCT u.phone) AS mobiles,
  1 AS weight
FROM profiles c
INNER JOIN profile_links g ON g.profile_id = c.id AND g.deleted_at IS NULL
INNER JOIN users u ON u.id = g.user_id AND u.deleted_at IS NULL
WHERE c.deleted_at IS NULL AND GREATEST(c.updated_at, g.updated_at, u.updated_at) > ?
GROUP BY c.id;
`
)

// LoaderConfig 提供 SQL 可配置能力
type LoaderConfig struct {
	FullSQL  string
	DeltaSQL string
}

// Loader 从业务库拉取档案联想候选
type Loader struct {
	db     *gorm.DB
	config LoaderConfig
}

// NewLoader 创建 Loader，SQL 为空时使用默认值
func NewLoader(db *gorm.DB, cfg LoaderConfig) *Loader {
	fullSQL := strings.TrimSpace(cfg.FullSQL)
	if fullSQL == "" {
		fullSQL = strings.TrimSpace(defaultFullSQL)
	}
	deltaSQL := strings.TrimSpace(cfg.DeltaSQL)
	if deltaSQL == "" {
		deltaSQL = strings.TrimSpace(defaultDeltaSQL)
	}

	return &Loader{
		db: db,
		config: LoaderConfig{
			FullSQL:  fullSQL,
			DeltaSQL: deltaSQL,
		},
	}
}

// Full 全量拉取
func (l *Loader) Full(ctx context.Context) ([]domainsuggest.ProfileCandidate, error) {
	return l.query(ctx, l.config.FullSQL)
}

// Delta 增量拉取，按时间过滤
func (l *Loader) Delta(ctx context.Context, since time.Time) ([]domainsuggest.ProfileCandidate, error) {
	if strings.TrimSpace(l.config.DeltaSQL) == "" {
		return nil, nil
	}
	return l.query(ctx, l.config.DeltaSQL, since)
}

type record struct {
	ID      int64   `gorm:"column:id"`
	Name    string  `gorm:"column:name"`
	Mobiles *string `gorm:"column:mobiles"`
	Weight  int     `gorm:"column:weight"`
}

func (l *Loader) query(ctx context.Context, sql string, args ...interface{}) ([]domainsuggest.ProfileCandidate, error) {
	if l.db == nil {
		return nil, fmt.Errorf("suggest loader db is nil")
	}

	var rows []record
	if err := l.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	candidates := make([]domainsuggest.ProfileCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, row.profileCandidate())
	}

	log.Infow("suggest loader finished query", "sql", sanitizeSQL(sql), "count", len(candidates))

	return candidates, nil
}

func (r record) profileCandidate() domainsuggest.ProfileCandidate {
	mobiles := ""
	if r.Mobiles != nil {
		mobiles = *r.Mobiles
	}
	return domainsuggest.NewProfileCandidate(r.ID, r.Name, splitMobiles(mobiles), r.Weight)
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
