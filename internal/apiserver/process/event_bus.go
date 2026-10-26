package process

import "fmt"

// prepareMessaging rejects retired assemblies before provisioning, DB or SDK resources.
func (s *apiServer) prepareMessaging() error {
	if s == nil || s.cfg == nil || s.cfg.Options == nil {
		return fmt.Errorf("explicit messaging configuration required")
	}
	_, finalOwner := s.cfg.Messaging.Transactions["iam.mysql"]
	if s.cfg.Messaging.SelectedProvider() != "rocketmq" || len(s.cfg.Messaging.Transactions) != 1 || !finalOwner || (s.cfg.Events != nil && s.cfg.Events.ReliableMessaging.Enabled) {
		return fmt.Errorf("final RocketMQ transaction owner required; NSQ and publishing Outbox/Relay are retired")
	}
	return s.cfg.Messaging.Validate()
}
