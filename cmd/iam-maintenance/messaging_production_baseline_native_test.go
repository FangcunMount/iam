//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/messagingcutover"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

// Exercise both real pre-installation versions, with the original audit DDL.
// No runtime, SDK, Redis or Broker is constructed by these CLI invocations.
func TestM3MessagingOfflineProductionBaselinesNative(t *testing.T) {
	require.Equal(t, "true", os.Getenv("GITHUB_ACTIONS"))
	cfg, err := driver.ParseDSN(os.Getenv("IAM_M3_TEST_MYSQL_DSN"))
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Addr)
	auditSQL, err := os.ReadFile("../../internal/pkg/migration/migrations/000041_message_failure_audit.up.sql")
	require.NoError(t, err)
	for _, baseline := range []uint{40, 41} {
		t.Run(fmt.Sprintf("schema%d", baseline), func(t *testing.T) {
			connector, err := driver.NewConnector(cfg)
			require.NoError(t, err)
			admin := sql.OpenDB(connector)
			t.Cleanup(func() { require.NoError(t, admin.Close()) })
			random := make([]byte, 12)
			_, err = rand.Read(random)
			require.NoError(t, err)
			name := "iam_m3_offline_" + hex.EncodeToString(random)
			_, err = admin.Exec("CREATE DATABASE `" + name + "`")
			require.NoError(t, err)
			t.Cleanup(func() { _, err := admin.Exec("DROP DATABASE `" + name + "`"); require.NoError(t, err) })
			owned := *cfg
			owned.DBName, owned.MultiStatements = name, true
			connector, err = driver.NewConnector(&owned)
			require.NoError(t, err)
			db := sql.OpenDB(connector)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			_, err = db.Exec(`CREATE TABLE schema_migrations(version BIGINT NOT NULL PRIMARY KEY,dirty BOOLEAN NOT NULL);
CREATE TABLE rm_outbox(id INT PRIMARY KEY,state VARBINARY(24) NOT NULL);
CREATE TABLE domain_event_outbox(id INT PRIMARY KEY,status VARBINARY(24) NOT NULL);
CREATE TABLE retained_authn(id INT PRIMARY KEY,value INT NOT NULL);
INSERT INTO retained_authn VALUES(1,1);`)
			require.NoError(t, err)
			_, err = db.Exec("INSERT INTO schema_migrations VALUES(?,FALSE)", baseline)
			require.NoError(t, err)
			insertAudit := func() {
				_, err := db.Exec(`INSERT INTO iam_message_failure_audit
(identity_hash,provider,physical_topic,original_wire,wire_hash,topic,channel_name,application_id,first_transport_id,last_transport_id,metadata_json,metadata_hash,payload,payload_hash,first_cause,last_cause,attempts,source_timestamp_ns,first_seen_unix_ms,last_seen_unix_ms)
VALUES(UNHEX(REPEAT('01',32)),'nsq','owned',X'00',UNHEX(REPEAT('02',32)),'owned','owned','owned','first','last','{}',UNHEX(REPEAT('03',32)),X'00',UNHEX(REPEAT('04',32)),'owned','owned',1,1,1,1)`)
				require.NoError(t, err)
			}
			if baseline == 41 {
				_, err = db.Exec(string(auditSQL))
				require.NoError(t, err)
				insertAudit() // Preexisting terminal audit facts are outside M3 restore scope.
			}
			setMessagingOfflineDatabaseEnvironment(t, cfg, name)
			directory := t.TempDir()
			require.NoError(t, os.Chmod(directory, 0700))
			preview := func(file string, args ...string) (string, []byte) {
				require.NoError(t, run(args, &bytes.Buffer{}))
				raw, err := os.ReadFile(file)
				require.NoError(t, err)
				h := sha256.Sum256(raw)
				return hex.EncodeToString(h[:]), raw
			}
			protection := filepath.Join(directory, "protection.json")
			if baseline == 40 {
				_, err = db.Exec(string(auditSQL))
				require.NoError(t, err)
				require.Error(t, run([]string{"m3-messaging-recovery", "--store-id", "owned-primary", "--writer-epoch", "m3-1", "--export-protection", protection}, &bytes.Buffer{}), "a table predating its journal cannot be claimed")
				_, err = db.Exec("DROP TABLE iam_message_failure_audit")
				require.NoError(t, err)
			}
			protectionSHA, raw := preview(protection, "m3-messaging-recovery", "--store-id", "owned-primary", "--writer-epoch", "m3-1", "--export-protection", protection)
			p, err := messagingcutover.DecodeProtection(raw, protectionSHA)
			require.NoError(t, err)
			require.Equal(t, baseline, p.Before.Version)
			if baseline == 40 {
				require.Equal(t, "eight-new-objects-and-original-version-journal", p.Scope)
			} else {
				require.Equal(t, "seven-new-objects-and-original-version-journal", p.Scope)
			}
			schema := filepath.Join(directory, "schema.json")
			schemaSHA, _ := preview(schema, "m3-messaging", "--store-id", "owned-primary", "--writer-epoch", "m3-1", "--export-plan", schema)
			require.NoError(t, run([]string{"m3-messaging", "--apply", "--writers-excluded", "--plan", schema, "--reviewed-sha256", schemaSHA}, &bytes.Buffer{}))
			var version uint
			var dirty bool
			require.NoError(t, db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty))
			require.EqualValues(t, 45, version)
			require.False(t, dirty)
			var auditDDLName, auditDDL string
			require.NoError(t, db.QueryRow("SHOW CREATE TABLE iam_message_failure_audit").Scan(&auditDDLName, &auditDDL))
			restore := []string{"m3-messaging-recovery", "--restore", "--writers-excluded", "--pre-start-restore-attested", "--protection", protection, "--reviewed-sha256", protectionSHA}
			if baseline == 40 {
				insertAudit()
				require.Error(t, run(restore, &bytes.Buffer{}), "new failure evidence must never be erased")
				var count int
				require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM iam_message_failure_audit").Scan(&count))
				require.Equal(t, 1, count)
				_, err = db.Exec("DELETE FROM iam_message_failure_audit") // Only this test's explicitly owned row.
				require.NoError(t, err)
			}
			_, err = db.Exec("UPDATE retained_authn SET value=2 WHERE id=1")
			require.NoError(t, err)
			require.NoError(t, run(restore, &bytes.Buffer{}))
			require.NoError(t, db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty))
			require.Equal(t, baseline, version)
			require.False(t, dirty)
			var value int
			require.NoError(t, db.QueryRow("SELECT value FROM retained_authn WHERE id=1").Scan(&value))
			require.Equal(t, 2, value, "restore must preserve post-protection AuthN facts")
			if baseline == 40 {
				var count int
				require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_message_failure_audit'").Scan(&count))
				require.Zero(t, count)
				// A partially applied original migration41 may leave a dirty journal.
				_, err = db.Exec(auditDDL)
				require.NoError(t, err)
				_, err = db.Exec("UPDATE schema_migrations SET version=41,dirty=TRUE")
				require.NoError(t, err)
				require.NoError(t, run(restore, &bytes.Buffer{}))
				require.NoError(t, db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty))
				require.EqualValues(t, 40, version)
				require.False(t, dirty)
			} else {
				var count int
				require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM iam_message_failure_audit").Scan(&count))
				require.Equal(t, 1, count, "schema41's original failure evidence must remain")
			}
			_, err = messagingcutover.Read(context.Background(), db)
			require.NoError(t, err)
		})
	}
}

func setMessagingOfflineDatabaseEnvironment(t *testing.T, cfg *driver.Config, name string) {
	t.Helper()
	host, port, err := net.SplitHostPort(cfg.Addr)
	require.NoError(t, err)
	for key, value := range map[string]string{"HOST": host, "PORT": port, "USERNAME": cfg.User, "PASSWORD": cfg.Passwd, "DATABASE": name, "TLS_MODE": "disabled"} {
		t.Setenv("IAM_APISERVER_MYSQL_"+key, value)
	}
}
