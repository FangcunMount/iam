package policychange

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/reliable-messaging/message"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
)

const PolicyBranch = "policy_version"
const PolicyScope = "scope:global"

var ErrInvalidChangePlan = errors.New("invalid frozen authorization change plan")

// CommandIntent binds a stable business input to the authenticated subject,
// operation and authorization scope. Generated event IDs, versions and wire are
// intentionally excluded from its input hash. Constructors perform no I/O.
type CommandIntent struct {
	key   string
	hash  [32]byte
	input []byte
}

func NewCommandIntent(operation, subject, scope, key string, input any) (CommandIntent, error) {
	for _, v := range []string{operation, subject, scope, key} {
		if v == "" || !utf8.ValidString(v) || strings.TrimSpace(v) != v || len(v) > 255 {
			return CommandIntent{}, ErrInvalidChangePlan
		}
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > 256*1024 || len(raw) == 0 || !utf8.Valid(raw) || raw[0] != '{' || opaqueInput(reflect.ValueOf(input), 0) {
		return CommandIntent{}, ErrInvalidChangePlan
	}
	// Store the original tuple, rather than using a digest as identity. JSON array
	// framing distinguishes embedded delimiters and scopes; SDK compares raw keys.
	encoded, err := json.Marshal([]string{"iam-authz-operation/v1", subject, scope, operation, key})
	if err != nil || len(encoded) > 1024 {
		return CommandIntent{}, ErrInvalidChangePlan
	}
	h := sha256.New()
	h.Write([]byte("iam-authz-input/v1\x00"))
	for _, v := range [][]byte{[]byte(operation), []byte(subject), []byte(scope), raw} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(v)))
		h.Write(size[:])
		h.Write(v)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return CommandIntent{key: string(encoded), hash: sum, input: bytes.Clone(raw)}, nil
}
func (c CommandIntent) Key() string         { return c.key }
func (c CommandIntent) InputHash() [32]byte { return c.hash }
func (c CommandIntent) Input() []byte       { return bytes.Clone(c.input) }
func (c CommandIntent) valid() bool         { return c.key != "" && len(c.input) > 0 }

// FreezeVersionPlan is called only after read-only business planning establishes
// an actual change. The host supplies the original identity/time exactly once,
// before half messages or its root transaction. No-op planning must skip it.
func FreezeVersionPlan(command CommandIntent, expected int64, operationID, eventID string, at time.Time, logical, physical string) (transaction.FrozenPlan, error) {
	if !command.valid() || expected < 1 || expected == math.MaxInt64 || at.IsZero() {
		return transaction.FrozenPlan{}, ErrInvalidChangePlan
	}
	body, err := json.Marshal(struct {
		Version int64 `json:"version"`
	}{expected + 1})
	if err != nil {
		return transaction.FrozenPlan{}, err
	}
	m, err := message.New(message.Input{Producer: "iam", ID: eventID, Destination: logical, EventType: eventing.AuthzVersionChanged, SchemaVersion: "v2", Scope: PolicyScope, ContentType: "application/json", OccurredAt: at.Format(time.RFC3339Nano), Payload: body})
	if err != nil {
		return transaction.FrozenPlan{}, err
	}
	wire, err := legacy.Encode(legacy.Envelope{UUID: eventID, Payload: body, Metadata: eventing.PolicyVersionWireMetadata(expected + 1)}, legacy.Revision1)
	if err != nil {
		return transaction.FrozenPlan{}, err
	}
	return transaction.NewPlan(transaction.PlanInput{Owner: "iam.mysql", Scope: PolicyScope, OperationID: operationID, IdempotencyKey: command.key, InputHash: command.hash, Branches: []transaction.BranchInput{{ID: PolicyBranch, Message: m, Wire: wire, PhysicalTopic: physical, RouteRevision: "m3-1"}}})
}

// VersionPlan reads the reservation WINNER without allocating an event or
// recomputing its first wire. Expected/Target describe the required local CAS.
type VersionPlan struct {
	Expected, Target int64
	Message          message.Message
	artifacts        *LocalArtifacts
	eventPayload     []byte
}

func DecodeVersionPlan(plan transaction.FrozenPlan) (VersionPlan, error) {
	if !plan.Valid() {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	p := plan.Input()
	if p.Owner != "iam.mysql" || p.Scope != PolicyScope || len(p.Branches) != 1 {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	b := p.Branches[0]
	in := b.Message.Input()
	eventPayload, artifacts, decodeErr := decodeLocalArtifacts(in)
	if decodeErr != nil {
		return VersionPlan{}, decodeErr
	}
	var body struct {
		Version int64 `json:"version"`
	}
	d := json.NewDecoder(bytes.NewReader(eventPayload))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || body.Version < 2 || in.Producer != "iam" || in.EventType != eventing.AuthzVersionChanged || in.SchemaVersion != "v2" || in.Scope != PolicyScope || b.ID != PolicyBranch || b.RouteRevision != "m3-1" {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	canonical, _ := json.Marshal(body)
	if !bytes.Equal(canonical, eventPayload) {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	outer, ok, err := legacy.Decode(b.Wire)
	if err != nil || !ok || outer.UUID != in.ID || !bytes.Equal(outer.Payload, eventPayload) || !reflect.DeepEqual(outer.Metadata, eventing.PolicyVersionWireMetadata(body.Version)) {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	// This producer freezes the original revision-one wire byte-for-byte. Do not
	// accept alternate envelopes then rewrite them on an observed duplicate.
	wire, err := legacy.Encode(outer, legacy.Revision1)
	if err != nil || !bytes.Equal(wire, b.Wire) {
		return VersionPlan{}, ErrInvalidChangePlan
	}
	return VersionPlan{Expected: body.Version - 1, Target: body.Version, Message: b.Message, artifacts: artifacts, eventPayload: eventPayload}, nil
}
func MatchCommand(plan transaction.FrozenPlan, command CommandIntent) error {
	if !command.valid() {
		return ErrInvalidChangePlan
	}
	if err := plan.MatchRequest("iam.mysql", PolicyScope, command.key, command.hash); err != nil {
		return err
	}
	_, err := DecodeVersionPlan(plan)
	return err
}

// Opaque domain values must be projected explicitly. encoding/json would silently
// erase private Scope fields, making different tenant/store commands hash alike.
func opaqueInput(value reflect.Value, depth int) bool {
	if depth > 64 {
		return true
	}
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return false
		}
		return opaqueInput(value.Elem(), depth+1)
	}
	if value.CanInterface() {
		if _, ok := value.Interface().(json.Marshaler); ok {
			return false
		}
	}
	switch value.Kind() {
	case reflect.String:
		return !utf8.ValidString(value.String())
	case reflect.Pointer:
		if value.IsNil() {
			return false
		}
		return opaqueInput(value.Elem(), depth+1)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			if field.Tag.Get("json") == "-" {
				continue
			}
			if field.PkgPath != "" || opaqueInput(value.Field(i), depth+1) {
				return true
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if opaqueInput(value.Index(i), depth+1) {
				return true
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			if key.Kind() == reflect.String && !utf8.ValidString(key.String()) {
				return true
			}
			if opaqueInput(value.MapIndex(key), depth+1) {
				return true
			}
		}
	}
	return false
}
