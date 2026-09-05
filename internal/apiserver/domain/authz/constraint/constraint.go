package constraint

import (
	"encoding/json"
	"sort"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

const Version1 uint32 = 1

type Operator string

const OperatorEQ Operator = "eq"

type Value struct {
	Type   attribute.Type `json:"type"`
	String *string        `json:"string,omitempty"`
	Int64  *int64         `json:"int64,omitempty"`
	Bool   *bool          `json:"bool,omitempty"`
}

func StringValue(value string) Value {
	return Value{Type: attribute.TypeString, String: &value}
}

func Int64Value(value int64) Value {
	return Value{Type: attribute.TypeInt64, Int64: &value}
}

func BoolValue(value bool) Value {
	return Value{Type: attribute.TypeBool, Bool: &value}
}

func (v Value) Validate() error { return validateValue(v) }

type Predicate struct {
	Key      string   `json:"key"`
	Operator Operator `json:"operator"`
	Value    Value    `json:"value"`
}

func Equal(key string, value Value) Predicate {
	return Predicate{Key: key, Operator: OperatorEQ, Value: value}
}

type Set struct {
	Version uint32      `json:"version"`
	AllOf   []Predicate `json:"all_of"`
}

// Attributes is the normalized internal representation of trusted, typed
// object attributes supplied by the business service that owns the object.
type Attributes map[string]Value

// Evaluation is the fail-closed result of evaluating one all_of set.
type Evaluation struct {
	Matched              bool
	MissingAttributeKeys []string
}

func Empty() Set {
	return Set{Version: Version1, AllOf: []Predicate{}}
}

func New(predicates ...Predicate) (Set, error) {
	return Set{Version: Version1, AllOf: predicates}.Normalize()
}

func (s Set) Normalize() (Set, error) { return s.normalize(true) }

// Read-only evaluation may borrow scalar values; exported normalized values own them.
func (s Set) normalize(ownValues bool) (Set, error) {
	if s.Version != 0 && s.Version != Version1 {
		return Set{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported constraint set version: %d", s.Version)
	}
	if len(s.AllOf) > 8 {
		return Set{}, perrors.WithCode(code.ErrInvalidArgument, "constraint set supports at most 8 predicates")
	}
	normalized := make([]Predicate, len(s.AllOf))
	copy(normalized, s.AllOf)
	seen := make(map[string]struct{}, len(normalized))
	for index := range normalized {
		predicate, err := normalizePredicate(normalized[index])
		if err != nil {
			return Set{}, err
		}
		if _, exists := seen[predicate.Key]; exists {
			return Set{}, perrors.WithCode(code.ErrInvalidArgument, "duplicate constraint attribute: %s", predicate.Key)
		}
		seen[predicate.Key] = struct{}{}
		if ownValues {
			predicate.Value = predicate.Value.Clone()
		}
		normalized[index] = predicate
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Key < normalized[j].Key })
	return Set{Version: Version1, AllOf: normalized}, nil
}

func (s Set) IsUnconditional() bool {
	normalized, err := s.normalize(false)
	return err == nil && len(normalized.AllOf) == 0
}

func (s Set) ValidateAgainst(schema attribute.Schema) error {
	normalized, err := s.normalize(false)
	if err != nil {
		return err
	}
	normalizedSchema, err := schema.Normalize()
	if err != nil {
		return err
	}
	for _, predicate := range normalized.AllOf {
		definition, ok := normalizedSchema.Find(predicate.Key)
		if !ok {
			return perrors.WithCode(code.ErrInvalidArgument, "unsupported constraint attribute: %s", predicate.Key)
		}
		if definition.Type != predicate.Value.Type {
			return perrors.WithCode(code.ErrInvalidArgument, "constraint attribute type mismatch: %s", predicate.Key)
		}
		if predicate.Value.Type == attribute.TypeString && len(definition.AllowedStringValues) > 0 {
			allowed := false
			for _, candidate := range definition.AllowedStringValues {
				if predicate.Value.String != nil && candidate == *predicate.Value.String {
					allowed = true
					break
				}
			}
			if !allowed {
				return perrors.WithCode(code.ErrInvalidArgument, "constraint attribute value is not allowed: %s", predicate.Key)
			}
		}
	}
	return nil
}

// Evaluate applies all predicates to trusted, typed object attributes.
// Missing keys are a normal deny result; malformed or type-incompatible
// values are contract errors.
func (s Set) Evaluate(attributes Attributes) (Evaluation, error) {
	normalized, err := s.normalize(false)
	if err != nil {
		return Evaluation{}, err
	}
	if len(normalized.AllOf) == 0 {
		return Evaluation{Matched: true}, nil
	}

	missing := make([]string, 0)
	for _, predicate := range normalized.AllOf {
		actual, ok := attributes[predicate.Key]
		if !ok {
			missing = append(missing, predicate.Key)
			continue
		}
		if err := validateValue(actual); err != nil {
			return Evaluation{}, perrors.WithCode(code.ErrInvalidArgument, "invalid object attribute %s: %v", predicate.Key, err)
		}
		if actual.Type != predicate.Value.Type {
			return Evaluation{}, perrors.WithCode(code.ErrInvalidArgument, "object attribute type mismatch: %s", predicate.Key)
		}
		if !valuesEqual(actual, predicate.Value) {
			return Evaluation{Matched: false}, nil
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Evaluation{Matched: false, MissingAttributeKeys: missing}, nil
	}
	return Evaluation{Matched: true}, nil
}

func (s Set) CanonicalJSON() ([]byte, error) {
	normalized, err := s.normalize(false)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func ParseJSON(data []byte) (Set, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return Empty(), nil
	}
	var set Set
	if err := json.Unmarshal(data, &set); err != nil {
		return Set{}, perrors.WithCode(code.ErrInvalidArgument, "invalid constraint set JSON: %v", err)
	}
	return set.Normalize()
}

func normalizePredicate(predicate Predicate) (Predicate, error) {
	predicate.Key = strings.TrimSpace(predicate.Key)
	if !strings.HasPrefix(predicate.Key, "object.") || len(predicate.Key) == len("object.") {
		return Predicate{}, perrors.WithCode(code.ErrInvalidArgument, "constraint key must use object.<name>: %s", predicate.Key)
	}
	if predicate.Operator != OperatorEQ {
		return Predicate{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported constraint operator: %s", predicate.Operator)
	}
	if err := validateValue(predicate.Value); err != nil {
		return Predicate{}, err
	}
	return predicate, nil
}

func validateValue(value Value) error {
	setValues := 0
	if value.String != nil {
		setValues++
	}
	if value.Int64 != nil {
		setValues++
	}
	if value.Bool != nil {
		setValues++
	}
	if setValues != 1 {
		return perrors.WithCode(code.ErrInvalidArgument, "constraint value must contain exactly one typed value")
	}
	switch value.Type {
	case attribute.TypeString:
		if value.String == nil {
			return perrors.WithCode(code.ErrInvalidArgument, "constraint string value is required")
		}
	case attribute.TypeInt64:
		if value.Int64 == nil {
			return perrors.WithCode(code.ErrInvalidArgument, "constraint int64 value is required")
		}
	case attribute.TypeBool:
		if value.Bool == nil {
			return perrors.WithCode(code.ErrInvalidArgument, "constraint bool value is required")
		}
	default:
		return perrors.WithCode(code.ErrInvalidArgument, "unsupported constraint value type: %s", value.Type)
	}
	return nil
}

func valuesEqual(left, right Value) bool {
	if left.Type != right.Type {
		return false
	}
	switch left.Type {
	case attribute.TypeString:
		return left.String != nil && right.String != nil && *left.String == *right.String
	case attribute.TypeInt64:
		return left.Int64 != nil && right.Int64 != nil && *left.Int64 == *right.Int64
	case attribute.TypeBool:
		return left.Bool != nil && right.Bool != nil && *left.Bool == *right.Bool
	default:
		return false
	}
}

func (v Value) Clone() Value {
	out := v
	if v.String != nil {
		value := *v.String
		out.String = &value
	}
	if v.Int64 != nil {
		value := *v.Int64
		out.Int64 = &value
	}
	if v.Bool != nil {
		value := *v.Bool
		out.Bool = &value
	}
	return out
}
func (s Set) Clone() Set {
	out := s
	if s.AllOf != nil {
		out.AllOf = make([]Predicate, len(s.AllOf))
		copy(out.AllOf, s.AllOf)
	}
	for i := range out.AllOf {
		out.AllOf[i].Value = out.AllOf[i].Value.Clone()
	}
	return out
}
