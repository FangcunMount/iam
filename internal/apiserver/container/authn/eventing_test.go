package authn

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
	jwksmysql "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/jwks"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/sms"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthnModuleInitializeSMSMQRequiresCatalogPublisher(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)

	module := NewAuthnModule()
	err := module.InitializeWithDeps(authnEventingDeps(t, db, redisClient, nil))

	require.Error(t, err)
	require.ErrorContains(t, err, "requires the catalog event publisher")
}

func TestAuthnModuleInitializeRejectsUnknownSMSProvider(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)
	deps := authnEventingDeps(t, db, redisClient, &capturingEventPublisher{})
	deps.SMS.Provider = "aliun"

	module := NewAuthnModule()
	err := module.InitializeWithDeps(deps)

	require.Error(t, err)
	require.ErrorContains(t, err, "unknown sms.provider")
}

func TestAuthnModuleSMSMQUsesCatalogBackedPublisher(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)

	publisher := &capturingEventPublisher{}
	module := NewAuthnModule()
	require.NoError(t, module.InitializeWithDeps(authnEventingDeps(t, db, redisClient, publisher)))
	caps := module.ApplicationCapabilities()
	require.NotNil(t, caps.LoginPhoneOTPSender)

	require.NoError(t, caps.LoginPhoneOTPSender.SendLoginPhoneOTP(context.Background(), "13800138000"))

	require.Len(t, publisher.events, 1)
	require.Equal(t, eventing.LoginOTPSMS, publisher.events[0].EventType())
	payload, ok := publisher.events[0].Payload().(sms.LoginOTPSMSPayload)
	require.True(t, ok)
	require.Equal(t, sms.EventLoginOTPSMS, payload.EventType)
	require.Equal(t, "login", payload.Scene)
	require.Equal(t, "+8613800138000", payload.PhoneE164)
	require.NotEmpty(t, payload.Code)
}

func setupAuthnEventingTest(t *testing.T) (*gorm.DB, *goredis.Client) {
	t.Helper()
	t.Setenv("TZ", "Asia/Shanghai")

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&jwksmysql.KeyPO{}))

	mr := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	return db, redisClient
}

func authnEventingDeps(t *testing.T, db *gorm.DB, redisClient *goredis.Client, publisher event.Publisher) AuthnModuleDeps {
	t.Helper()
	smsOptions := *apiserveroptions.NewSMSOptions()
	smsOptions.Provider = "mq"
	smsOptions.LoginOTPTTL = 5 * time.Minute
	smsOptions.LoginOTPSendCooldown = time.Minute
	smsOptions.LoginOTPCodeLength = 6
	jwksOptions := *apiserveroptions.NewJWKSOptions()
	jwksOptions.AutoInit = true
	jwksOptions.KeysDir = t.TempDir()
	return AuthnModuleDeps{
		DB:             db,
		RedisClient:    redisClient,
		EventPublisher: publisher,
		Environment:    genericapiserver.EnvironmentTest,
		Auth:           *apiserveroptions.NewAuthOptions(),
		JWKS:           jwksOptions,
		SMS:            smsOptions,
	}
}

type capturingEventPublisher struct {
	events []event.DomainEvent
}

func (p *capturingEventPublisher) Publish(_ context.Context, evt event.DomainEvent) error {
	if evt != nil {
		p.events = append(p.events, evt)
	}
	return nil
}

func (p *capturingEventPublisher) PublishAll(ctx context.Context, events []event.DomainEvent) error {
	for _, evt := range events {
		if err := p.Publish(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}
