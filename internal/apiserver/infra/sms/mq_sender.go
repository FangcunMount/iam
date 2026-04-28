package sms

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/google/uuid"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
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
	publisher messaging.Publisher
	topic     string
}

var _ authentication.SMSSender = (*MQLoginOTPSender)(nil)

// NewMQLoginOTPSender 使用 EventBus 的 Publisher 发布登录 OTP 短信任务
func NewMQLoginOTPSender(bus messaging.EventBus, topic string) *MQLoginOTPSender {
	if topic == "" {
		topic = LoginOTPSMSTopicDefault
	}
	return &MQLoginOTPSender{
		publisher: bus.Publisher(),
		topic:     topic,
	}
}

// SendLoginOTP 发布一条 MQ 消息，由下游完成实际发送
func (s *MQLoginOTPSender) SendLoginOTP(ctx context.Context, phoneE164, code string) error {
	p := LoginOTPSMSPayload{
		EventType: EventLoginOTPSMS,
		Scene:     "login",
		PhoneE164: phoneE164,
		Code:      code,
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal login otp sms payload: %w", err)
	}
	msg := messaging.NewMessage(uuid.New().String(), payload)
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}
	msg.Metadata["event_type"] = EventLoginOTPSMS
	if err := s.publisher.PublishMessage(ctx, s.topic, msg); err != nil {
		return fmt.Errorf("publish login otp sms: %w", err)
	}
	return nil
}
