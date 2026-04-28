// Package messaging 消息基础设施层
// 基于 component-base/pkg/messaging 实现策略版本通知
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/google/uuid"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

const (
	// AuthzVersionTopic 授权版本变更主题
	AuthzVersionTopic = "iam.authz.version"

	// AuthzVersionChannel 订阅通道（用于负载均衡，同组消费者共享）
	AuthzVersionChannel = "iam-policy-sync"
)

// VersionNotifier NSQ 版本通知器实现
type VersionNotifier struct {
	publisher  messaging.Publisher
	subscriber messaging.Subscriber
	mu         sync.RWMutex
	closed     bool
	stopOnce   sync.Once
}

var _ domain.VersionNotifier = (*VersionNotifier)(nil)

// VersionChangeMessage 版本变更消息
type VersionChangeMessage struct {
	TenantID string `json:"tenant_id"`
	Version  int64  `json:"version"`
}

// NewVersionNotifier 创建版本通知器
func NewVersionNotifier(bus messaging.EventBus) domain.VersionNotifier {
	return &VersionNotifier{
		publisher:  bus.Publisher(),
		subscriber: bus.Subscriber(),
		closed:     false,
	}
}

// NewVersionNotifierWithPubSub 使用独立的 Publisher/Subscriber 创建
func NewVersionNotifierWithPubSub(publisher messaging.Publisher, subscriber messaging.Subscriber) domain.VersionNotifier {
	return &VersionNotifier{
		publisher:  publisher,
		subscriber: subscriber,
		closed:     false,
	}
}

// Publish 发布策略版本变更通知
func (n *VersionNotifier) Publish(ctx context.Context, tenantID string, version int64) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.closed {
		return fmt.Errorf("notifier is closed")
	}

	msg := VersionChangeMessage{
		TenantID: tenantID,
		Version:  version,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.ErrorContext(ctx, "failed to marshal version message",
			log.String("error", err.Error()),
		)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 创建带 UUID 的消息
	message := messaging.NewMessage(uuid.New().String(), payload)
	message.Metadata["tenant_id"] = tenantID

	if err := n.publisher.PublishMessage(ctx, AuthzVersionTopic, message); err != nil {
		log.ErrorContext(ctx, "failed to publish version change",
			log.String("topic", AuthzVersionTopic),
			log.String("tenant_id", tenantID),
			log.String("error", err.Error()),
		)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.InfoContext(ctx, "version change published",
		log.String("topic", AuthzVersionTopic),
		log.String("tenant_id", tenantID),
		log.Int64("version", version),
	)
	return nil
}

// Subscribe 订阅策略版本变更通知
func (n *VersionNotifier) Subscribe(ctx context.Context, handler domain.VersionChangeHandler) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return fmt.Errorf("notifier is closed")
	}

	// 使用 messaging.Handler 包装领域处理函数
	msgHandler := func(ctx context.Context, msg *messaging.Message) error {
		var changeMsg VersionChangeMessage
		if err := json.Unmarshal(msg.Payload, &changeMsg); err != nil {
			log.WarnContext(ctx, "failed to unmarshal version message",
				log.String("error", err.Error()),
				log.String("uuid", msg.UUID),
			)
			// 消息格式错误，不重试，直接 Ack
			return nil
		}

		log.DebugContext(ctx, "version change received",
			log.String("tenant_id", changeMsg.TenantID),
			log.Int64("version", changeMsg.Version),
			log.String("uuid", msg.UUID),
		)

		// 调用领域处理函数
		handler(changeMsg.TenantID, changeMsg.Version)
		return nil
	}

	// 订阅主题
	if err := n.subscriber.Subscribe(AuthzVersionTopic, AuthzVersionChannel, msgHandler); err != nil {
		log.ErrorContext(ctx, "failed to subscribe to authz version topic",
			log.String("topic", AuthzVersionTopic),
			log.String("channel", AuthzVersionChannel),
			log.String("error", err.Error()),
		)
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.InfoContext(ctx, "subscribed to authz version topic",
		log.String("topic", AuthzVersionTopic),
		log.String("channel", AuthzVersionChannel),
	)
	return nil
}

// Close 关闭订阅
func (n *VersionNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil
	}

	n.closed = true

	var errs []error

	// 停止订阅
	n.stopOnce.Do(func() {
		if n.subscriber != nil {
			n.subscriber.Stop()
		}
	})

	log.Info("version notifier closed",
		log.String("topic", AuthzVersionTopic),
	)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
