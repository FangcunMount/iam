// Command m6_nsq_message_probe verifies two physical deliveries of the same
// application message on a disposable NSQ proof channel.
package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/nsqio/go-nsq"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	address := os.Getenv("RM_IAM_M6_PROBE_NSQD")
	identity := os.Getenv("RM_IAM_M6_PROBE_ID")
	if address == "" || identity == "" {
		return fmt.Errorf("disposable NSQ address and application identity required")
	}
	consumer, err := nsq.NewConsumer("iam.authz.version.v2", "m6-unknown-proof", nsq.NewConfig())
	if err != nil {
		return err
	}
	consumer.SetLogger(nil, nsq.LogLevelError)
	received := make(chan *nsq.Message, 2)
	consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		copy := *message
		copy.Body = append([]byte(nil), message.Body...)
		received <- &copy
		return nil
	}))
	if err := consumer.ConnectToNSQD(address); err != nil {
		return err
	}
	defer func() { consumer.Stop(); <-consumer.StopChan }()
	var messages [2]*nsq.Message
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	for i := range messages {
		select {
		case messages[i] = <-received:
		case <-deadline.C:
			return fmt.Errorf("timed out after %d physical deliveries", i)
		}
	}
	if messages[0].ID == messages[1].ID || !bytes.Equal(messages[0].Body, messages[1].Body) {
		return fmt.Errorf("physical NSQ IDs must differ while original wire bytes match")
	}
	for i, message := range messages {
		envelope, recognized, err := legacy.Decode(message.Body)
		if err != nil || !recognized || envelope.UUID != identity {
			return fmt.Errorf("delivery %d has wrong original identity: recognized=%t uuid=%q err=%v", i, recognized, envelope.UUID, err)
		}
	}
	fmt.Printf("PASS two NSQ physical IDs retain original application ID %s and identical wire bytes\n", identity)
	return nil
}
