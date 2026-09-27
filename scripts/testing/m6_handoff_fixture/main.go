// Command m6_handoff_fixture writes a disposable host fact and SDK Outbox
// intent in one MySQL transaction for the IAM image handoff isolation job.
// It must never be pointed at a production database.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/FangcunMount/reliable-messaging/message"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
	_ "github.com/go-sql-driver/mysql"
)

const identity = "m6-standard-handoff-fixture"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("committed host fact and standard intent:", identity)
}

func run() error {
	dsn := os.Getenv("RM_IAM_M6_FIXTURE_DSN")
	if dsn == "" {
		return fmt.Errorf("disposable MySQL DSN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE m6_handoff_fact (
  id VARBINARY(128) NOT NULL PRIMARY KEY,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB`); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO m6_handoff_fact(id, created_at) VALUES (?, ?)`, identity, now.Format("2006-01-02 15:04:05.000000")); err != nil {
		return err
	}
	intent, err := message.New(message.Input{
		Producer: "iam", ID: identity, Destination: "iam.authz.version.v2",
		EventType: "iam.authz.version_changed.v2", SchemaVersion: "v2",
		Scope: "scope:global", ContentType: "application/json",
		OccurredAt: now.Format(time.RFC3339Nano), Payload: []byte(`{"version":1}`),
	})
	if err != nil {
		return err
	}
	appender, err := sdkmysql.Bind(tx)
	if err != nil {
		return err
	}
	if err := appender.Append(ctx, intent, now); err != nil {
		return err
	}
	return tx.Commit()
}
