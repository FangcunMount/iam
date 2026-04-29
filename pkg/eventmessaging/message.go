package eventmessaging

import (
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/pkg/event"
	"github.com/FangcunMount/iam/pkg/eventcodec"
)

// BuildMessage adapts a domain event into component-base messaging.Message.
func BuildMessage(evt event.DomainEvent, source string) (*messaging.Message, error) {
	payload, err := eventcodec.EncodePayload(evt)
	if err != nil {
		return nil, err
	}
	msg := messaging.NewMessage(evt.EventID(), payload)
	msg.Metadata = eventcodec.MetadataFromEvent(evt, source)
	return msg, nil
}
