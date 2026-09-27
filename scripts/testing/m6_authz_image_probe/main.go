// Command m6_authz_image_probe commits real IAM authorization facts and
// standard intents, then checks decisions through a separately running IAM
// image. It only accepts disposable fixture inputs.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const baseResourceKey = "qs:assessment:collection:m6-image-probe"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("RM_IAM_M6_AUTHZ_DSN")
	target := os.Getenv("RM_IAM_M6_AUTHZ_GRPC")
	if dsn == "" || target == "" {
		return fmt.Errorf("disposable IAM MySQL and gRPC targets required")
	}
	probeID := os.Getenv("RM_IAM_M6_AUTHZ_ID")
	if probeID == "" {
		probeID = "base"
	}
	if probeID != "base" && probeID != "unknown" {
		return fmt.Errorf("unsupported disposable authorization probe identity %q", probeID)
	}
	resourceKey := baseResourceKey
	roleKey := "qs:m6-image-probe"
	if probeID == "unknown" {
		resourceKey += "-unknown"
		roleKey += "-unknown"
	}
	armFile := os.Getenv("RM_IAM_M6_AUTHZ_ARM_FILE")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	config, err := eventcatalog.Load("/app/configs/events.yaml")
	if err != nil {
		return err
	}
	stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(config))
	if err != nil {
		return err
	}
	uow := authzuow.NewUnitOfWork(db, nil, stager)
	if err := db.WithContext(ctx).Exec("INSERT IGNORE INTO users(id,status) VALUES(2,1)").Error; err != nil {
		return err
	}
	var initialCount int64
	if err := db.WithContext(ctx).Table("rm_outbox").Count(&initialCount).Error; err != nil {
		return err
	}
	client, closeClient, err := newClient(target)
	if err != nil {
		return err
	}
	defer closeClient()
	probeRole, err := role.NewRole(roleKey, "IAM image message proof")
	if err != nil {
		return err
	}
	probeResource, err := resource.NewResource(resourceKey, []string{"read"}, resource.WithDisplayName("IAM image message proof"))
	if err != nil {
		return err
	}
	var grant permissiongrant.Grant
	version, err := commitVersion(ctx, uow, "m6-image-allow", func(txctx context.Context, repos appuow.TxRepositories) error {
		if err := repos.Roles.Create(txctx, &probeRole); err != nil {
			return err
		}
		if err := repos.Resources.Create(txctx, &probeResource); err != nil {
			return err
		}
		var err error
		grant, err = permissiongrant.New(probeRole.ID, probeResource.ID, probeResource.KeyString(), "read", "m6-image-proof")
		if err != nil {
			return err
		}
		if err := repos.PermissionGrants.Create(txctx, &grant); err != nil {
			return err
		}
		assigned, err := assignment.NewAssignment(assignment.SubjectType("user"), meta.FromUint64(2), probeRole.ID, assignment.WithGrantedBy("m6-image-proof"))
		if err != nil {
			return err
		}
		return repos.Assignments.Create(txctx, &assigned)
	})
	if err != nil {
		return fmt.Errorf("commit allow facts and intent: %w", err)
	}
	if err := waitDecision(ctx, client, resourceKey, version, true); err != nil {
		return fmt.Errorf("allow decision: %w", err)
	}
	fmt.Printf("IAM image allowed real role grant at policy version %d\n", version)
	if armFile != "" {
		if err := os.WriteFile(armFile, []byte("revoke-publish"), 0o600); err != nil {
			return fmt.Errorf("arm disposable NSQ confirmation-loss proxy: %w", err)
		}
	}
	version, err = commitVersion(ctx, uow, "m6-image-revoke", func(txctx context.Context, repos appuow.TxRepositories) error {
		_, err := repos.PermissionGrants.AtomicRevoke(txctx, grant.ID)
		return err
	})
	if err != nil {
		return fmt.Errorf("commit revoke and intent: %w", err)
	}
	if err := waitDecision(ctx, client, resourceKey, version, false); err != nil {
		return fmt.Errorf("deny decision: %w", err)
	}
	fmt.Printf("IAM image denied revoked grant at policy version %d\n", version)
	if armFile != "" {
		if err := waitRetryWait(ctx, db); err != nil {
			return fmt.Errorf("revoke notification confirmation loss: %w", err)
		}
	}
	version, err = commitVersion(ctx, uow, "m6-image-restore", func(txctx context.Context, repos appuow.TxRepositories) error {
		restored, err := permissiongrant.New(probeRole.ID, probeResource.ID, probeResource.KeyString(), "read", "m6-image-restore")
		if err != nil {
			return err
		}
		return repos.PermissionGrants.Create(txctx, &restored)
	})
	if err != nil {
		return fmt.Errorf("commit restore and intent: %w", err)
	}
	if err := waitDecision(ctx, client, resourceKey, version, true); err != nil {
		return fmt.Errorf("restored decision: %w", err)
	}
	fmt.Printf("IAM image restored grant at policy version %d\n", version)
	if err := waitPublished(ctx, db, initialCount+3); err != nil {
		return err
	}
	if err := waitDecision(ctx, client, resourceKey, version, true); err != nil {
		return fmt.Errorf("restored decision after delivery recovery: %w", err)
	}
	fmt.Printf("PASS IAM image allow -> deny -> allow on committed policy versions; three same-transaction intents published (Outbox %d -> %d)\n", initialCount, initialCount+3)
	return nil
}

func waitRetryWait(ctx context.Context, db *gorm.DB) error {
	for {
		var count int64
		if err := db.WithContext(ctx).Table("rm_outbox").Where("state='retry_wait' AND failure_count=1").Count(&count).Error; err != nil {
			return err
		}
		if count == 1 {
			fmt.Println("IAM revoke intent retained retry_wait after NSQ accepted but PUB OK was lost")
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("revoke intent never entered retry_wait: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitPublished(ctx context.Context, db *gorm.DB, expected int64) error {
	for {
		var total, published int64
		if err := db.WithContext(ctx).Table("rm_outbox").Count(&total).Error; err != nil {
			return err
		}
		if err := db.WithContext(ctx).Table("rm_outbox").Where("state='published'").Count(&published).Error; err != nil {
			return err
		}
		if total != expected {
			return fmt.Errorf("authorization intent count: got %d, want %d", total, expected)
		}
		if published == total {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("authorization intents not fully published: total=%d published=%d: %w", total, published, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func commitVersion(ctx context.Context, uow appuow.UnitOfWork, reason string, change func(context.Context, appuow.TxRepositories) error) (int64, error) {
	var version int64
	err := uow.WithinTx(ctx, func(txctx context.Context, repos appuow.TxRepositories) error {
		if err := change(txctx, repos); err != nil {
			return err
		}
		updated, err := repos.PolicyVersions.Increment(txctx, "m6-image-proof", reason)
		if err != nil {
			return err
		}
		version = updated.Version
		return repos.Events.Stage(txctx, policy.NewVersionChangedEvent(version))
	})
	return version, err
}

func newClient(target string) (authzv4.AuthorizationServiceClient, func(), error) {
	cert, err := tls.LoadX509KeyPair(os.Getenv("RM_IAM_M6_AUTHZ_CERT"), os.Getenv("RM_IAM_M6_AUTHZ_KEY"))
	if err != nil {
		return nil, nil, err
	}
	caPEM, err := os.ReadFile(os.Getenv("RM_IAM_M6_AUTHZ_CA"))
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, nil, fmt.Errorf("disposable CA certificate invalid")
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		RootCAs: roots, Certificates: []tls.Certificate{cert}, ServerName: "iam-apiserver", MinVersion: tls.VersionTLS12,
	})))
	if err != nil {
		return nil, nil, err
	}
	return authzv4.NewAuthorizationServiceClient(conn), func() { _ = conn.Close() }, nil
}

func waitDecision(parent context.Context, client authzv4.AuthorizationServiceClient, resourceKey string, version int64, allowed bool) error {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	var last string
	for {
		checkCtx, stop := context.WithTimeout(ctx, 2*time.Second)
		decision, err := client.Check(checkCtx, &authzv4.CheckRequest{
			Subject: "user:2", Resource: resourceKey, Action: "read",
		})
		stop()
		if err == nil && decision.PolicyVersion >= version && decision.Allowed == allowed {
			return nil
		}
		last = fmt.Sprintf("decision=%v err=%v", decision, err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("policy version %d allowed=%t not observed: %s: %w", version, allowed, last, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}
