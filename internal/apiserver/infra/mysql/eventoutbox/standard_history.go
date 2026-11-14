package eventoutbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const standardOutboxTable = "rm_outbox"
const standardHistoryTable = "iam_rm_outbox_history"
const legacyOutboxTable = "domain_event_outbox"
const legacyHistoryTable = "iam_domain_event_outbox_history"

func standardReadTable(db *gorm.DB) (string, error) {
	table, err := retainedReadTable(db, standardOutboxTable, standardHistoryTable)
	if err != nil {
		return "", err
	}
	if table == standardHistoryTable {
		if err = checkReliableSchema(db, table); err != nil {
			return "", err
		}
	}
	return table, nil
}

func legacyReadTable(db *gorm.DB) (string, error) {
	table, err := retainedReadTable(db, legacyOutboxTable, legacyHistoryTable)
	if err != nil {
		return "", err
	}
	if table == legacyHistoryTable {
		// Verify the complete retained record is readable, including the original payload.
		err = db.Exec(`SELECT id,event_id,event_type,aggregate_type,aggregate_id,topic_name,
			payload_json,status,attempt_count,next_attempt_at,last_error,created_at,updated_at,published_at
			FROM iam_domain_event_outbox_history LIMIT 0`).Error
		if err != nil {
			return "", err
		}
	}
	return table, nil
}

// Only the fixed original and retained names of the given Outbox are eligible.
// Rename occurs separately from these read-only paths.
func retainedReadTable(db *gorm.DB, original, history string) (string, error) {
	var objects []struct {
		Name string
		Kind string
	}
	err := db.Raw(`SELECT TABLE_NAME AS name, TABLE_TYPE AS kind
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND BINARY TABLE_NAME IN (?, ?)`,
		original, history).Scan(&objects).Error
	if err != nil {
		return "", err
	}
	if len(objects) != 1 || objects[0].Kind != "BASE TABLE" {
		return "", fmt.Errorf("exactly one IAM Outbox base table is required for %s", original)
	}
	if objects[0].Name == original {
		return original, nil
	}
	if objects[0].Name != history {
		return "", errors.New("unexpected IAM Outbox history name")
	}
	// information_schema hides objects the account cannot see. Require the
	// server's absent-table error, rather than treating permission loss as absence.
	_, err = db.Statement.ConnPool.ExecContext(db.Statement.Context, "SELECT id FROM "+original+" LIMIT 0")
	var missing *mysql.MySQLError
	if !errors.As(err, &missing) || missing.Number != 1146 {
		return "", errors.Join(fmt.Errorf("original IAM Outbox absence is unverified for %s", original), err)
	}
	return history, nil
}

func readOutboxSnapshot(ctx context.Context, db *gorm.DB, read func(*gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(read, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

// CheckTransactionHandoff keeps both retained publication histories in the
// startup gate. Operational writer exclusion is a separate requirement.
func CheckTransactionHandoff(ctx context.Context, db *gorm.DB) error {
	return readOutboxSnapshot(ctx, db, func(tx *gorm.DB) error {
		table, err := standardReadTable(tx)
		if err != nil {
			return err
		}
		legacy, err := legacyReadTable(tx)
		if err != nil {
			return err
		}
		for _, source := range []struct{ table, state string }{
			{legacy, "status"},
			{table, "state"},
		} {
			var found []uint64
			err = tx.Table(source.table).Select("id").
				Where(source.state+" IS NULL OR BINARY "+source.state+" <> ?", "published").
				Limit(1).Find(&found).Error
			if err != nil {
				return err
			}
			if len(found) != 0 {
				return ErrUnsafeMessagingHandoff
			}
		}
		return nil
	})
}
