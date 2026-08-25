package attribute

import (
	"sort"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type Type string

const (
	TypeString Type = "string"
	TypeInt64  Type = "int64"
	TypeBool   Type = "bool"
)

const ObjectOriginType = "object.origin_type"

type Definition struct {
	Key                 string   `json:"key"`
	Type                Type     `json:"type"`
	AllowedStringValues []string `json:"allowed_string_values,omitempty"`
}

type Schema struct {
	Version    uint32       `json:"version"`
	Attributes []Definition `json:"attributes"`
}

func EmptySchema() Schema {
	return Schema{Version: 1, Attributes: []Definition{}}
}

func NewSchema(definitions []Definition) (Schema, error) {
	if len(definitions) > 32 {
		return Schema{}, perrors.WithCode(code.ErrInvalidArgument, "attribute schema supports at most 32 attributes")
	}
	normalized := make([]Definition, len(definitions))
	copy(normalized, definitions)
	seen := make(map[string]struct{}, len(normalized))
	for index := range normalized {
		definition, err := normalizeDefinition(normalized[index])
		if err != nil {
			return Schema{}, err
		}
		if _, exists := seen[definition.Key]; exists {
			return Schema{}, perrors.WithCode(code.ErrInvalidArgument, "duplicate attribute definition: %s", definition.Key)
		}
		seen[definition.Key] = struct{}{}
		normalized[index] = definition
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Key < normalized[j].Key })
	return Schema{Version: 1, Attributes: normalized}, nil
}

func AssessmentSchema() Schema {
	schema, _ := NewSchema([]Definition{{
		Key:                 ObjectOriginType,
		Type:                TypeString,
		AllowedStringValues: []string{"adhoc", "plan"},
	}})
	return schema
}

func (s Schema) Normalize() (Schema, error) {
	if s.Version != 0 && s.Version != 1 {
		return Schema{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported attribute schema version: %d", s.Version)
	}
	return NewSchema(s.Attributes)
}

func (s Schema) Find(key string) (Definition, bool) {
	key = strings.TrimSpace(key)
	for _, definition := range s.Attributes {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

func normalizeDefinition(definition Definition) (Definition, error) {
	definition.Key = strings.TrimSpace(definition.Key)
	if !strings.HasPrefix(definition.Key, "object.") || len(definition.Key) == len("object.") {
		return Definition{}, perrors.WithCode(code.ErrInvalidArgument, "attribute key must use object.<name>: %s", definition.Key)
	}
	switch definition.Type {
	case TypeString:
	case TypeInt64, TypeBool:
		if len(definition.AllowedStringValues) > 0 {
			return Definition{}, perrors.WithCode(code.ErrInvalidArgument, "allowed string values require string attribute: %s", definition.Key)
		}
	default:
		return Definition{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported attribute type: %s", definition.Type)
	}
	seen := make(map[string]struct{}, len(definition.AllowedStringValues))
	values := make([]string, 0, len(definition.AllowedStringValues))
	for _, value := range definition.AllowedStringValues {
		value = strings.TrimSpace(value)
		if value == "" {
			return Definition{}, perrors.WithCode(code.ErrInvalidArgument, "allowed attribute value cannot be empty: %s", definition.Key)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Strings(values)
	definition.AllowedStringValues = values
	return definition, nil
}
