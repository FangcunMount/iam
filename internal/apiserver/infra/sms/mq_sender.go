package sms

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/messaging"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/pkg/event"
	"github.com/FangcunMount/iam/pkg/eventcodec"
)

// LoginOTPSMSTopicDefault NSQ topic：下游消费者（短信网关等）订阅并真正发送短信
const LoginOTPSMSTopicDefault = "iam.notify.sms"

// LoginOTPSMSPayload 登录 OTP 短信投递消息体（与具体厂商解耦）
type LoginOTPSMSPayload struct {
	EventType string `json:"event_type"` // EventLoginOTPSMS
	Scene     string `json:"scene"`      // login
	PhoneE164 string `json:"phone_e164"`
	Code      string `json:"code"`
}

// EventLoginOTPSMS 与 LoginOTPSMSPayload.event_type 一致，供消费者筛选
const EventLoginOTPSMS = "iam.login_otp_sms"

// MQLoginOTPSender 通过消息队列投递「待发短信」意图，不直连运营商
type MQLoginOTPSender struct {
	publisher event.Publisher
}

var _ authentication.SMSSender = (*MQLoginOTPSender)(nil)

// NewMQLoginOTPSender 使用 EventBus 的 Publisher 发布登录 OTP 短信任务。
// Deprecated: 新装配应使用 catalog-backed NewMQLoginOTPSenderWithPublisher；
// topic 参数仅保留给迁移窗口内的 legacy sms.mq.topic fallback。
func NewMQLoginOTPSender(bus messaging.EventBus, topic string) *MQLoginOTPSender {
	if topic == "" {
		topic = LoginOTPSMSTopicDefault
	}
	var publisher messaging.Publisher
	if bus != nil {
		publisher = bus.Publisher()
	}
	return &MQLoginOTPSender{
		publisher: legacyLoginOTPPublisher{publisher: publisher, topic: topic},
	}
}

func NewMQLoginOTPSenderWithPublisher(publisher event.Publisher) *MQLoginOTPSender {
	return &MQLoginOTPSender{publisher: publisher}
}

// SendLoginOTP 发布一条 MQ 消息，由下游完成实际发送
func (s *MQLoginOTPSender) SendLoginOTP(ctx context.Context, phoneE164, code string) error {
	if s == nil || s.publisher == nil {
		return fmt.Errorf("publish login otp sms: event publisher is not configured")
	}
	if err := s.publisher.Publish(ctx, NewLoginOTPSMSEvent(phoneE164, code)); err != nil {
		return fmt.Errorf("publish login otp sms: %w", err)
	}
	return nil
}

type LoginOTPSMSEvent struct {
	event.BaseEvent
	payload LoginOTPSMSPayload
}

func NewLoginOTPSMSEvent(phoneE164, code string) LoginOTPSMSEvent {
	payload := LoginOTPSMSPayload{
		EventType: EventLoginOTPSMS,
		Scene:     "login",
		PhoneE164: phoneE164,
		Code:      code,
	}
	return LoginOTPSMSEvent{
		BaseEvent: event.NewBaseEvent(eventing.LoginOTPSMS, "LoginOTP", phoneE164),
		payload:   payload,
	}
}

func (e LoginOTPSMSEvent) Payload() any {
	return e.payload
}

type legacyLoginOTPPublisher struct {
	publisher messaging.Publisher
	topic     string
}

func (p legacyLoginOTPPublisher) Publish(ctx context.Context, evt event.DomainEvent) error {
	if p.publisher == nil {
		return fmt.Errorf("legacy login otp publisher is not configured")
	}
	if evt == nil {
		return nil
	}
	payload, err := json.Marshal(evt.Payload())
	if err != nil {
		return fmt.Errorf("marshal login otp sms payload: %w", err)
	}
	msg := messaging.NewMessage(evt.EventID(), payload)
	msg.Metadata = eventcodec.MetadataFromEvent(evt, eventing.SourceAPIServer)
	return p.publisher.PublishMessage(ctx, p.topic, msg)
}

func (p legacyLoginOTPPublisher) PublishAll(ctx context.Context, events []event.DomainEvent) error {
	for _, evt := range events {
		if err := p.Publish(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}
