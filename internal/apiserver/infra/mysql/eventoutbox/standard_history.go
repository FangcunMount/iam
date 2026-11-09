package eventoutbox

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const standardOutboxTable = "rm_outbox"
const standardHistoryTable = "iam_rm_outbox_history"

// standardReadTable resolves one fixed base table. The archive is an atomic
// rename of the original, performed only in a separately approved window.
func standardReadTable(db *gorm.DB) (string, error) {
	var objects []struct {
		Name string
		Kind string
	}
	err := db.Raw(`SELECT TABLE_NAME AS name, TABLE_TYPE AS kind
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND BINARY TABLE_NAME IN (?, ?)`,
		standardOutboxTable, standardHistoryTable).Scan(&objects).Error
	if err != nil {
		return "", err
	}
	if len(objects) != 1 || objects[0].Kind != "BASE TABLE" {
		return "", errors.New("exactly one IAM standard Outbox base table is required")
	}
	if objects[0].Name == standardOutboxTable {
		return standardOutboxTable, nil
	}
	if objects[0].Name != standardHistoryTable {
		return "", errors.New("unexpected IAM standard Outbox history name")
	}
	// information_schema hides objects the account cannot see. Require the
	// server's absent-table error, rather than treating permission loss as absence.
	_, err = db.Statement.ConnPool.ExecContext(db.Statement.Context, "SELECT id FROM rm_outbox LIMIT 0")
	var missing *mysql.MySQLError
	if !errors.As(err, &missing) || missing.Number != 1146 {
		return "", errors.Join(errors.New("original IAM standard Outbox absence is unverified"), err)
	}
	if err = checkReliableSchema(db, standardHistoryTable); err != nil {
		return "", err
	}
	return standardHistoryTable, nil
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
		for _, source := range []struct{ table, state string }{
			{"domain_event_outbox", "status"},
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
