package sessionrevocation

import (
	"context"
	"errors"
	"fmt"
	"time"

	app "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/sessionrevocation"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusFailed     = "failed"
	StatusCompleted  = "completed"
)

type Task struct {
	TaskID        uint64     `gorm:"column:task_id;primaryKey;autoIncrement"`
	UserID        meta.ID    `gorm:"column:user_id"`
	UserVersion   uint32     `gorm:"column:user_version"`
	Action        string     `gorm:"column:action"`
	Reason        string     `gorm:"column:reason"`
	Status        string     `gorm:"column:status"`
	AttemptCount  uint32     `gorm:"column:attempt_count"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at"`
	LastError     *string    `gorm:"column:last_error"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
	CompletedAt   *time.Time `gorm:"column:completed_at"`
}

func (Task) TableName() string { return "identity_session_revocation_outbox" }

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

var _ app.Stager = (*Store)(nil)

func (s *Store) Stage(ctx context.Context, userID meta.ID, action, reason string) error {
	if s == nil || s.db == nil {
		return errors.New("session revocation store is unavailable")
	}
	var version uint32
	if err := s.db.WithContext(ctx).Table("users").
		Select("version").
		Where("id = ?", userID.Uint64()).
		Scan(&version).Error; err != nil {
		return err
	}
	if version == 0 {
		return fmt.Errorf("user version is unavailable")
	}
	task := Task{
		UserID:        userID,
		UserVersion:   version,
		Action:        action,
		Reason:        reason,
		Status:        StatusPending,
		NextAttemptAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&task).Error
}

func (s *Store) Claim(ctx context.Context, limit int, staleBefore time.Time) ([]Task, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("session revocation store is unavailable")
	}
	var claimed []Task
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&Task{}).
			Where("status = ? AND updated_at < ?", StatusProcessing, staleBefore).
			Updates(map[string]any{"status": StatusPending, "next_attempt_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND next_attempt_at <= ?", []string{StatusPending, StatusFailed}, now).
			Order("task_id ASC").
			Limit(limit).
			Find(&claimed).Error; err != nil {
			return err
		}
		for i := range claimed {
			if err := tx.Model(&Task{}).Where("task_id = ?", claimed[i].TaskID).
				Updates(map[string]any{
					"status":        StatusProcessing,
					"attempt_count": gorm.Expr("attempt_count + 1"),
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			claimed[i].Status = StatusProcessing
			claimed[i].AttemptCount++
		}
		return nil
	})
	return claimed, err
}

func (s *Store) Complete(ctx context.Context, taskID uint64) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&Task{}).Where("task_id = ? AND status = ?", taskID, StatusProcessing).
		Updates(map[string]any{
			"status":       StatusCompleted,
			"completed_at": now,
			"last_error":   nil,
			"updated_at":   now,
		}).Error
}

func (s *Store) Fail(ctx context.Context, taskID uint64, next time.Time) error {
	category := "session_revoke_failed"
	return s.db.WithContext(ctx).Model(&Task{}).Where("task_id = ? AND status = ?", taskID, StatusProcessing).
		Updates(map[string]any{
			"status":          StatusFailed,
			"next_attempt_at": next.UTC(),
			"last_error":      category,
			"updated_at":      time.Now().UTC(),
		}).Error
}

func (s *Store) OldestUnfinishedAge(ctx context.Context, now time.Time) (time.Duration, error) {
	var createdAt *time.Time
	err := s.db.WithContext(ctx).Model(&Task{}).
		Select("MIN(created_at)").
		Where("status <> ?", StatusCompleted).
		Scan(&createdAt).Error
	if err != nil || createdAt == nil {
		return 0, err
	}
	return now.Sub(createdAt.UTC()), nil
}
