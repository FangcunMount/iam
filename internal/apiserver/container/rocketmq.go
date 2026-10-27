package container

import (
	"context"
	"errors"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	"github.com/FangcunMount/iam/v5/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/messagefailure"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
)

func (c *Container) initRocketMQPolicySync(module *authz.AuthzModule) error {
	if c.runtimeOptions.Messaging.SelectedProvider() != "rocketmq" {
		return errors.New("final RocketMQ policy subscriber required; NSQ is retired")
	}
	if c.mysqlDB == nil {
		return errors.New("RocketMQ policy subscriber requires IAM MySQL")
	}
	pool, err := c.mysqlDB.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = messagefailure.CheckRocketMQAudit(ctx, pool); err != nil {
		return err
	}
	config, err := c.runtimeOptions.Messaging.RocketMQ([]string{policypublication.Topic}, 16, 5*time.Second)
	if err != nil {
		return err
	}
	audit, err := messagefailure.NewRocketMQ(pool, config.Routes, authz.ChannelPrefix+"."+c.runtimeOptions.Messaging.InstanceID)
	if err != nil {
		return err
	}
	s, err := rocketmq.NewSubscriber(rocketmq.SubscriberConfig{Config: config, MaxAttempts: 8, InvisibleDuration: 60 * time.Second, HandlerTimeout: 30 * time.Second, RetryDelay: 30 * time.Second})
	if err != nil {
		return err
	}
	c.sdkPolicySync = module.RocketMQPolicySyncSubscriber(s, audit.RecordRocketMQ, authz.ChannelPrefix+"."+c.runtimeOptions.Messaging.InstanceID)
	if c.sdkPolicySync == nil {
		return errors.New("RocketMQ policy subscriber dependencies unavailable")
	}
	return nil
}
