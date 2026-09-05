package authz

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

const ChannelPrefix = "iam-policy-sync"

func CurrentInstanceChannel() string {
	hostname, _ := os.Hostname()
	return InstanceChannel(hostname, os.Getpid())
}

func InstanceChannel(hostname string, pid int) string {
	host := sanitizeChannelPart(hostname)
	if host == "" {
		host = "unknown"
	}
	if pid <= 0 {
		pid = os.Getpid()
	}
	return fmt.Sprintf("%s.%s.%d#ephemeral", ChannelPrefix, host, pid)
}

func sanitizeChannelPart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-_.")
}
