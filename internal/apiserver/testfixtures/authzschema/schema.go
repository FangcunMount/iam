// Package authzschema provides business independent authorization examples.
package authzschema

import "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"

const AttributeKey = "object.status"

func Schema() attribute.Schema {
	s, err := attribute.NewSchema([]attribute.Definition{{Key: AttributeKey, Type: attribute.TypeString, AllowedStringValues: []string{"active", "paused"}}})
	if err != nil {
		panic(err)
	}
	return s
}
