package eventcodec

import (
	"github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/internal/pkg/event"
	sharedcodec "github.com/FangcunMount/iam/pkg/eventcodec"
	"github.com/FangcunMount/iam/pkg/eventmessaging"
)

func EncodePayload(evt event.DomainEvent) ([]byte, error) {
	return sharedcodec.EncodePayload(evt)
}

func MetadataFromEvent(evt event.DomainEvent, source string) map[string]string {
	return sharedcodec.MetadataFromEvent(evt, source)
}

func BuildMessage(evt event.DomainEvent, source string) (*messaging.Message, error) {
	return eventmessaging.BuildMessage(evt, source)
}
