package assembler

import (
	"context"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
	"github.com/FangcunMount/iam/pkg/event"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthnModuleInitializeSMSMQRequiresEventBusEvenWithPublisher(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)

	module := NewAuthnModule()
	err := module.InitializeWithDeps(authnEventingDeps(db, redisClient, nil, &capturingEventPublisher{}))

	require.Error(t, err)
	require.ErrorContains(t, err, "sms.provider=mq requires EventBus")
}

func TestAuthnModuleSMSMQUsesCatalogBackedPublisherWhenEventBusAvailable(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)

	publisher := &capturingEventPublisher{}
	module := NewAuthnModule()
	require.NoError(t, module.InitializeWithDeps(authnEventingDeps(db, redisClient, eventBusStub{}, publisher)))
	require.NotNil(t, module.LoginPreparationService)

	require.NoError(t, module.LoginPreparationService.SendPhoneOTPForLogin(context.Background(), "13800138000"))

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

	mr := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	return db, redisClient
}

func authnEventingDeps(db *gorm.DB, redisClient *goredis.Client, eventBus cbmessaging.EventBus, publisher event.Publisher) AuthnModuleDeps {
	smsOptions := *apiserveroptions.NewSMSOptions()
	smsOptions.Provider = "mq"
	smsOptions.LoginOTPTTL = 5 * time.Minute
	smsOptions.LoginOTPSendCooldown = time.Minute
	smsOptions.LoginOTPCodeLength = 6
	return AuthnModuleDeps{
		DB:             db,
		RedisClient:    redisClient,
		EventBus:       eventBus,
		EventPublisher: publisher,
		AppMode:        "test",
		Auth:           *apiserveroptions.NewAuthOptions(),
		JWKS:           *apiserveroptions.NewJWKSOptions(),
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

type eventBusStub struct{}

func (eventBusStub) Publisher() cbmessaging.Publisher {
	return nil
}

func (eventBusStub) Subscriber() cbmessaging.Subscriber {
	return nil
}

func (eventBusStub) Router() *cbmessaging.Router {
	return nil
}

func (eventBusStub) Health() error {
	return nil
}

func (eventBusStub) Close() error {
	return nil
}
