package bootstrapdelivery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/message"
)

// Notifications owns the two policy publication facts of a fresh transaction
// bootstrap. It is prepared under the migration lock before either hook runs.
type Notifications struct {
	conn   *sql.Conn
	mapper *eventing.PolicyIntentMapper
	topic  string
}

func Prepare(ctx context.Context, conn *sql.Conn, catalog *eventcatalog.Catalog) (*Notifications, error) {
	mapper, err := eventing.NewPolicyIntentMapper(catalog)
	if err != nil {
		return nil, err
	}
	topic, _ := catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	var emptyLedger bool
	if err := conn.QueryRowContext(ctx, "SELECT NOT EXISTS (SELECT 1 FROM schema_migrations)").Scan(&emptyLedger); err != nil {
		return nil, err
	}
	if emptyLedger {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS iam_bootstrap_policy_notification (
			phase TINYINT UNSIGNED PRIMARY KEY CHECK (phase IN (33,35)),
			event_id VARBINARY(128) NOT NULL UNIQUE,
			input_bytes BLOB NOT NULL,
			fingerprint BINARY(32) NOT NULL,
			delivery_committed TINYINT UNSIGNED NOT NULL DEFAULT 0 CHECK (delivery_committed IN (0,1))
		) ENGINE=InnoDB`); err != nil {
			return nil, err
		}
	}
	var valid bool
	if err := conn.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()
		 AND TABLE_NAME='iam_bootstrap_policy_notification' AND ENGINE='InnoDB')=1
		AND (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
		 AND TABLE_NAME='iam_bootstrap_policy_notification')=5
		AND (SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE()
		 AND TABLE_NAME='iam_bootstrap_policy_notification' AND IS_NULLABLE='NO'
		 AND (COLUMN_NAME,COLUMN_TYPE) IN (('phase','tinyint unsigned'),('event_id','varbinary(128)'),
		 ('input_bytes','blob'),('fingerprint','binary(32)'),('delivery_committed','tinyint unsigned')))=5
		AND (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
		 AND TABLE_NAME='iam_bootstrap_policy_notification' AND NON_UNIQUE=0 AND SEQ_IN_INDEX=1
		 AND SUB_PART IS NULL AND ((INDEX_NAME='PRIMARY' AND COLUMN_NAME='phase')
		 OR (INDEX_NAME='event_id' AND COLUMN_NAME='event_id')))=2
		AND (SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE()
		 AND TABLE_NAME='iam_bootstrap_policy_notification')=2
		AND NOT EXISTS (SELECT 1 FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=DATABASE()
		 AND EVENT_OBJECT_TABLE='iam_bootstrap_policy_notification')`).Scan(&valid); err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid fresh bootstrap notification layout")
	}
	return &Notifications{conn: conn, mapper: mapper, topic: topic}, nil
}

func (n *Notifications) Stage(phase uint) event.Stager {
	return notificationStager{phase: phase, mapper: n.mapper}
}

type notificationStager struct {
	phase  uint
	mapper *eventing.PolicyIntentMapper
}

func (s notificationStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if (s.phase != 33 && s.phase != 35) || len(events) != 1 {
		return errors.New("fresh bootstrap requires one policy notification per stage")
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	msg, err := s.mapper.Message(events[0])
	if err != nil {
		return err
	}
	in := msg.Input()
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	sum := msg.Fingerprint()
	return tx.WithContext(ctx).Exec(`INSERT INTO iam_bootstrap_policy_notification
		(phase,event_id,input_bytes,fingerprint,delivery_committed) VALUES(?,?,?,?,0)`, s.phase, in.ID, raw, sum[:]).Error
}

func (n *Notifications) Verify(ctx context.Context, phase uint) error {
	var id string
	var raw, fingerprint []byte
	var committed uint
	err := n.conn.QueryRowContext(ctx, `SELECT event_id,input_bytes,fingerprint,delivery_committed
		FROM iam_bootstrap_policy_notification WHERE phase=?`, phase).Scan(&id, &raw, &fingerprint, &committed)
	if err != nil {
		return fmt.Errorf("read fresh bootstrap notification for stage %d: %w", phase, err)
	}
	var in message.Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return errors.New("invalid stored fresh bootstrap notification")
	}
	canonical, err := json.Marshal(in)
	if err != nil || !bytes.Equal(raw, canonical) || in.ID != id || in.Producer != "iam" || in.Destination != n.topic ||
		in.EventType != eventing.AuthzVersionChanged || in.SchemaVersion != "v2" || in.Scope != "scope:global" ||
		in.ContentType != "application/json" || committed > 1 {
		return errors.New("fresh bootstrap notification fields differ from their original format")
	}
	if _, err := eventing.PolicyPayloadVersion(in.Payload); err != nil {
		return err
	}
	msg, err := message.New(in)
	if err != nil {
		return err
	}
	sum := msg.Fingerprint()
	if !bytes.Equal(fingerprint, sum[:]) {
		return errors.New("fresh bootstrap notification fingerprint differs")
	}
	return nil
}
