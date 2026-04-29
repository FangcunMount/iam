// Package messaging 消息基础设施层
// 基于 component-base/pkg/messaging 实现策略版本通知订阅
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/messaging"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

const (
	// AuthzVersionTopic 授权版本变更主题
	AuthzVersionTopic = "iam.authz.version"

	// AuthzVersionChannel 订阅通道（用于负载均衡，同组消费者共享）
	AuthzVersionChannel = "iam-policy-sync"
)

// VersionNotifier NSQ 版本通知订阅器实现
type VersionNotifier struct {
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

// NewVersionNotifier 创建版本通知订阅器
func NewVersionNotifier(bus messaging.EventBus) domain.VersionNotifier {
	return &VersionNotifier{
		subscriber: bus.Subscriber(),
		closed:     false,
	}
}

// NewVersionNotifierWithPubSub 使用独立的 Publisher/Subscriber 创建。
// Publisher 参数保留用于兼容旧测试与装配入口；发布职责已迁移到 outbox relay。
func NewVersionNotifierWithPubSub(_ messaging.Publisher, subscriber messaging.Subscriber) domain.VersionNotifier {
	return &VersionNotifier{
		subscriber: subscriber,
		closed:     false,
	}
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
