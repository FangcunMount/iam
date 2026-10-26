package platform

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransactionEventingConstructorUsesExplicitCatalogAndStartsNoClient(t *testing.T) {
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	ca := filepath.Join(dir, "ca.pem")
	creds := filepath.Join(dir, "credentials.json")
	routes := filepath.Join(dir, "routes.json")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	require.NoError(t, os.WriteFile(creds, []byte(`{"username":"fixture","password":"fixture-only"}`), 0600))
	topic := "iam.authz.version.v2"
	physical, err := transport.PhysicalTopicV2(topic, transport.TransactionMessageType, "iam.mysql", "m3-1")
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"version": 2, "routes": []transport.RouteBinding{{Topic: topic, PhysicalTopic: physical, MessageType: transport.TransactionMessageType, Owner: "iam.mysql", RouteRevision: "m3-1"}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(routes, raw, 0600))
	db, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: new(sql.DB), SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	opts := options.MessagingOptions{Provider: "rocketmq", Endpoint: "127.0.0.1:1", CAFile: ca, CredentialsFile: creds, RoutesFile: routes, InstanceID: "fixture", Transactions: map[string]options.TransactionOptions{"iam.mysql": {StoreID: "fixture-primary", WriterEpoch: "m3-1", RouteRevision: "m3-1"}}}
	cfg, err := eventcatalog.Load(filepath.Join(testRepoRoot(t), "configs/events-m3.yaml"))
	require.NoError(t, err)
	deps := EventingDeps{DB: db, Messaging: opts}
	owner, err := NewTransactionEventing(deps, eventcatalog.NewCatalog(cfg), "")
	require.NoError(t, err)
	require.NotNil(t, owner.Writer)
	require.Nil(t, owner.normal, "disabled MQ SMS allocated another producer")
	require.False(t, owner.Stats().Ready)
	require.NoError(t, owner.Stop(context.Background()))
	old, err := eventcatalog.Load(filepath.Join(testRepoRoot(t), "configs/events.yaml"))
	require.NoError(t, err)
	_, err = NewTransactionEventing(deps, eventcatalog.NewCatalog(old), "")
	require.Error(t, err, "old Outbox catalog was silently treated as transaction delivery")
	mixed := deps
	mixed.ReliableMessaging.Enabled = true
	_, err = NewTransactionEventing(mixed, eventcatalog.NewCatalog(cfg), "")
	require.ErrorContains(t, err, "cannot both be enabled")
	deps.CatalogPath = filepath.Join(testRepoRoot(t), "configs/events-m3.yaml")
	assembled, err := InitEventing(deps)
	require.NoError(t, err)
	require.IsType(t, &TransactionEventing{}, assembled.ReliableRuntime)
	require.IsType(t, owner.Writer, assembled.Stager)
	require.NotNil(t, assembled.Publisher)
	require.NoError(t, assembled.ReliableRuntime.Stop(context.Background()))
	deps.EnableSMSMessaging = true
	_, err = NewTransactionEventing(deps, eventcatalog.NewCatalog(cfg), "")
	require.Error(t, err, "enabled NORMAL SMS accepted a transaction-only manifest")
}
