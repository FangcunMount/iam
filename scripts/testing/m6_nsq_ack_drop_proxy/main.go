// Command m6_nsq_ack_drop_proxy forwards NSQ TCP traffic in a disposable
// fixture and drops one PUB OK after the broker has accepted the message.
// It is not a production proxy.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const maxFrame = 64 << 20

var claimed atomic.Bool

func main() {
	upstream := os.Getenv("RM_IAM_M6_PROXY_UPSTREAM")
	armFile := os.Getenv("RM_IAM_M6_PROXY_ARM_FILE")
	topic := os.Getenv("RM_IAM_M6_PROXY_TOPIC")
	if upstream == "" || armFile == "" || topic == "" {
		log.Fatal("upstream, arm file, and topic are required")
	}
	listener, err := net.Listen("tcp", ":4150")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Print("NSQ test proxy ready")
	for {
		client, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handle(client, upstream, armFile, topic)
	}
}

func handle(client net.Conn, upstreamAddress, armFile, topic string) {
	defer client.Close()
	broker, err := net.DialTimeout("tcp", upstreamAddress, 5*time.Second)
	if err != nil {
		log.Printf("connect broker: %v", err)
		return
	}
	defer broker.Close()
	var dropOK atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := forwardCommands(broker, client, armFile, topic, &dropOK); err != nil && err != io.EOF {
			log.Printf("forward NSQ commands: %v", err)
		}
		_ = broker.Close()
	}()
	err = forwardFrames(client, broker, &dropOK)
	if err != nil && err != io.EOF {
		log.Printf("forward NSQ frames: %v", err)
	}
	_ = client.Close()
	_ = broker.Close()
	<-done
}

func forwardCommands(dst io.Writer, src io.Reader, armFile, topic string, dropOK *atomic.Bool) error {
	reader := bufio.NewReader(src)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(reader, magic); err != nil {
		return err
	}
	if string(magic) != "  V2" {
		return fmt.Errorf("unexpected NSQ magic %q", magic)
	}
	if _, err := dst.Write(magic); err != nil {
		return err
	}
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			return fmt.Errorf("empty NSQ command")
		}
		if _, err := dst.Write(line); err != nil {
			return err
		}
		command := fields[0]
		switch command {
		case "IDENTIFY", "AUTH", "PUB", "MPUB", "DPUB":
			var length [4]byte
			if _, err := io.ReadFull(reader, length[:]); err != nil {
				return err
			}
			n := binary.BigEndian.Uint32(length[:])
			if n > maxFrame {
				return fmt.Errorf("oversize NSQ command body: %d", n)
			}
			if command == "PUB" && len(fields) == 2 && fields[1] == topic {
				if _, err := os.Stat(armFile); err == nil && claimed.CompareAndSwap(false, true) {
					dropOK.Store(true)
				}
			}
			if _, err := dst.Write(length[:]); err != nil {
				return err
			}
			if _, err := io.CopyN(dst, reader, int64(n)); err != nil {
				return err
			}
		}
	}
}

func forwardFrames(dst io.Writer, src io.Reader, dropOK *atomic.Bool) error {
	for {
		var length [4]byte
		if _, err := io.ReadFull(src, length[:]); err != nil {
			return err
		}
		n := binary.BigEndian.Uint32(length[:])
		if n < 4 || n > maxFrame {
			return fmt.Errorf("invalid NSQ frame length: %d", n)
		}
		frame := make([]byte, n)
		if _, err := io.ReadFull(src, frame); err != nil {
			return err
		}
		if dropOK.Load() && binary.BigEndian.Uint32(frame[:4]) == 0 && string(frame[4:]) == "OK" {
			log.Print("DROPPED_PUB_OK_AFTER_BROKER_ACCEPT")
			return nil
		}
		if _, err := dst.Write(length[:]); err != nil {
			return err
		}
		if _, err := dst.Write(frame); err != nil {
			return err
		}
	}
}
