// Package assessment contains the QS contract used only by integration tests.
package assessment

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
)

const AttributeKey = "object.origin_type"
const Resource = "qs:evaluation:collection:assessments"
const Service = "qs-apiserver.svc"

func Schema() attribute.Schema {
	s, err := attribute.NewSchema([]attribute.Definition{{Key: AttributeKey, Type: attribute.TypeString, AllowedStringValues: []string{"adhoc", "plan"}}})
	if err != nil {
		panic(err)
	}
	return s
}
func Policy() *objectattributeadmission.Registry {
	p, err := objectattributeadmission.New([]objectattributeadmission.Provider{{Service: Service, Resource: Resource, Attributes: []string{AttributeKey}}})
	if err != nil {
		panic(err)
	}
	return p
}
