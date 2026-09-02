package sms

import (
	"context"
	"fmt"

	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

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

var _ challengeApp.SMSSender = (*MQLoginOTPSender)(nil)

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
