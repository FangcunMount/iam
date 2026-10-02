#!/usr/bin/env bash
# Capture only synthetic test values through the exact original wire codec.
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
source_commit=a929f5301ed8265a8f44726b474da27ae10cb1c1
[[ $(git -C "$repo" rev-parse "$source_commit^{commit}") == "$source_commit" ]] || exit 1
snapshot=$(mktemp -d "${TMPDIR:-/tmp}/iam-sms-wire.XXXXXX")
trap 'rm -rf -- "$snapshot"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
git -C "$repo" archive "$source_commit" | tar -x -C "$snapshot"
mkdir -p "$snapshot/cmd/sms-wire-capture"
cat > "$snapshot/cmd/sms-wire-capture/main.go" <<'GO'
package main
import (
 "os"
 "time"
 cb "github.com/FangcunMount/component-base/pkg/messaging"
 "github.com/FangcunMount/iam/v5/pkg/event"
 "github.com/FangcunMount/iam/v5/pkg/eventcodec"
)
func main() {
 evt := event.Event[map[string]string]{BaseEvent:event.BaseEvent{
  ID:"otp-event-1", EventTypeValue:"iam.login_otp_sms",
  OccurredAtValue:time.Date(2026,9,27,22,0,0,0,time.FixedZone("UTC+8",8*3600)),
  AggregateTypeValue:"LoginOTP", AggregateIDValue:"+8613800138000",
 },Data:map[string]string{"event_type":"iam.login_otp_sms","scene":"login","phone_e164":"+8613800138000","code":"123456"}}
 payload,err := eventcodec.EncodePayload(evt);if err!=nil {panic(err)}
 body,err := cb.EncodeMessagePayload(&cb.Message{UUID:evt.EventID(),Payload:payload,Metadata:eventcodec.MetadataFromEvent(evt,"iam-apiserver")});if err!=nil {panic(err)}
 if _,err=os.Stdout.Write(body);err!=nil {panic(err)}
}
GO
(cd "$snapshot" && GOWORK=off go run ./cmd/sms-wire-capture)
