package messagefailure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	mysqldriver "github.com/go-sql-driver/mysql"
)

// Audit stores terminal NSQ failures in IAM's own MySQL database. A successful
// Record is the only event that allows the SDK failure consumer to FIN.
type Audit struct{ db *sql.DB }

func New(db *sql.DB) (*Audit, error) {
	if db == nil {
		return nil, errors.New("IAM failure audit requires MySQL")
	}
	return &Audit{db: db}, nil
}

func (a *Audit) Record(ctx context.Context, failed legacy.FailedHandoff) error {
	if a == nil || a.db == nil {
		return errors.New("IAM failure audit unavailable")
	}
	if failed.Topic == "" || failed.Channel == "" || failed.UUID == "" || failed.Attempts < 1 || failed.Cause == "" {
		return errors.New("invalid terminal failure identity or cause")
	}
	metadata, err := json.Marshal(failed.Metadata)
	if err != nil {
		return fmt.Errorf("encode failure metadata: %w", err)
	}
	identity := sha256.Sum256([]byte(failed.Topic + "\x00" + failed.Channel + "\x00" + failed.UUID))
	payloadHash := sha256.Sum256(failed.Payload)
	metadataHash := sha256.Sum256(metadata)
	now := time.Now().UnixMilli() // Epoch values have no session-timezone ambiguity.
	_, err = a.db.ExecContext(ctx, `
INSERT INTO iam_nsq_failure_audit (
 identity_hash,topic,channel_name,application_id,first_transport_id,last_transport_id,
 metadata_json,metadata_hash,payload,payload_hash,first_cause,last_cause,attempts,
 source_timestamp_ns,first_seen_unix_ms,last_seen_unix_ms
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		identity[:], failed.Topic, failed.Channel, failed.UUID,
		failed.TransportMessageID, failed.TransportMessageID,
		metadata, metadataHash[:], failed.Payload, payloadHash[:],
		failed.Cause, failed.Cause, failed.Attempts, failed.Timestamp, now, now)
	if err == nil {
		return nil
	}
	var mysqlErr *mysqldriver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return fmt.Errorf("insert IAM terminal failure audit: %w", err)
	}
	// A confirmed handoff can be delivered more than once. Lock the existing
	// record and count a repeat only if its immutable business identity and
	// content agree. A collision or changed payload must not be silently FINed.
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin failure audit duplicate check: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var id uint64
	var topic, channel, applicationID string
	var existingPayloadHash, existingMetadataHash []byte
	if err := tx.QueryRowContext(ctx, `
SELECT id,topic,channel_name,application_id,payload_hash,metadata_hash
FROM iam_nsq_failure_audit WHERE identity_hash=? FOR UPDATE`, identity[:]).Scan(
		&id, &topic, &channel, &applicationID, &existingPayloadHash, &existingMetadataHash,
	); err != nil {
		return fmt.Errorf("read duplicate IAM failure audit: %w", err)
	}
	if topic != failed.Topic || channel != failed.Channel || applicationID != failed.UUID ||
		!bytes.Equal(existingPayloadHash, payloadHash[:]) || !bytes.Equal(existingMetadataHash, metadataHash[:]) {
		return fmt.Errorf("IAM failure identity reused with different content: topic=%q channel=%q application_id=%q", failed.Topic, failed.Channel, failed.UUID)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE iam_nsq_failure_audit
SET last_transport_id=?,last_cause=?,attempts=?,source_timestamp_ns=?,
    seen_count=seen_count+1,last_seen_unix_ms=?
WHERE id=?`, failed.TransportMessageID, failed.Cause, failed.Attempts, failed.Timestamp, now, id); err != nil {
		return fmt.Errorf("update duplicate IAM failure audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit duplicate IAM failure audit: %w", err)
	}
	return nil
}
