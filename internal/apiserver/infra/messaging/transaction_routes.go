package messaging

import (
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
)

// TrustedTransactionRoutes is inert and copies the owner's frozen capability
// map. Historical evidence never registers publishers, subscribers or Checkers.
func TrustedTransactionRoutes(config rocketmq.TransactionConfig) (*transport.RouteResolver, error) {
	convert := func(input []rocketmq.TransactionRoute) []transport.RouteBinding {
		rows := make([]transport.RouteBinding, 0, len(input))
		for _, r := range input {
			rows = append(rows, transport.RouteBinding{Topic: r.LogicalTopic, PhysicalTopic: r.PhysicalTopic, MessageType: transport.TransactionMessageType, Owner: config.Owner, RouteRevision: r.Revision, NamingScheme: r.NamingScheme})
		}
		return rows
	}
	if config.Owner != "iam.mysql" {
		return nil, transaction.ErrBinding
	}
	return transport.NewRouteResolver(convert(config.Routes), convert(config.HistoricalRoutes))
}

func ValidateRecordedTransactionRoutes(plan transaction.FrozenPlan, routes *transport.RouteResolver) error {
	return validateTransactionRoutes(plan, routes, false)
}
func ValidateActiveTransactionRoutes(plan transaction.FrozenPlan, routes *transport.RouteResolver) error {
	return validateTransactionRoutes(plan, routes, true)
}
func validateTransactionRoutes(plan transaction.FrozenPlan, routes *transport.RouteResolver, active bool) error {
	if !plan.Valid() || plan.Input().Owner != "iam.mysql" || routes == nil {
		return transaction.ErrBinding
	}
	for _, branch := range plan.Input().Branches {
		key := transport.RouteKey{Topic: branch.Message.Input().Destination, MessageType: transport.TransactionMessageType, Owner: "iam.mysql", RouteRevision: branch.RouteRevision}
		row, err := routes.RecordedRoute(key, branch.PhysicalTopic)
		if err != nil {
			return transaction.ErrBinding
		}
		if active {
			current, err := routes.PublishRoute(key)
			if err != nil || current.PhysicalTopic != row.PhysicalTopic {
				return transaction.ErrBinding
			}
		}
	}
	return nil
}
