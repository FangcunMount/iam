package assembler

import (
	"context"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthnModuleInitializeSMSMQRequiresEventBusEvenWithPublisher(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)
	viper.Set("sms.provider", "mq")

	module := NewAuthnModule()
	err := module.Initialize(db, redisClient, &capturingEventPublisher{})

	require.Error(t, err)
	require.ErrorContains(t, err, "sms.provider=mq requires EventBus")
}

func TestAuthnModuleSMSMQUsesCatalogBackedPublisherWhenEventBusAvailable(t *testing.T) {
	db, redisClient := setupAuthnEventingTest(t)
	viper.Set("sms.provider", "mq")

	publisher := &capturingEventPublisher{}
	module := NewAuthnModule()
	require.NoError(t, module.Initialize(db, redisClient, eventBusStub{}, publisher))
	require.NotNil(t, module.LoginPreparationService)

	require.NoError(t, module.LoginPreparationService.SendPhoneOTPForLogin(context.Background(), "13800138000"))

	require.Len(t, publisher.events, 1)
	require.Equal(t, eventcatalog.LoginOTPSMS, publisher.events[0].EventType())
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
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("jwks.auto_init", false)
	viper.Set("app.mode", "test")
	viper.Set("sms.login_otp_ttl", 5*time.Minute)
	viper.Set("sms.login_otp_send_cooldown", time.Minute)
	viper.Set("sms.login_otp_code_length", 6)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	return db, redisClient
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
