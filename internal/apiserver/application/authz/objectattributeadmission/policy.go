// Package objectattributeadmission decides which callers may provide trusted
// object attributes at the application boundary.
package objectattributeadmission

import (
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
)

const (
	AssessmentResource                = "qs:evaluation:collection:assessments"
	TrustedAssessmentAttributeService = "qs-apiserver.svc"
)

type Request struct {
	CallerService string
	ResourceKey   string
	AttributeKey  string
}

type Policy interface {
	AuthorizeAttribute(Request) error
}

type policy struct{}

func NewDefaultPolicy() Policy {
	return policy{}
}

func (policy) AuthorizeAttribute(request Request) error {
	key := strings.TrimSpace(request.AttributeKey)
	if key != attribute.ObjectOriginType || strings.TrimSpace(request.ResourceKey) != AssessmentResource {
		return ErrUnsupportedAttribute{Key: key}
	}
	if strings.TrimSpace(request.CallerService) != TrustedAssessmentAttributeService {
		return ErrUntrustedCaller{}
	}
	return nil
}

type ErrUnsupportedAttribute struct {
	Key string
}

func (e ErrUnsupportedAttribute) Error() string {
	return "unsupported object attribute: " + e.Key
}

type ErrUntrustedCaller struct{}

func (e ErrUntrustedCaller) Error() string {
	return "caller is not trusted to provide this object attribute"
}
