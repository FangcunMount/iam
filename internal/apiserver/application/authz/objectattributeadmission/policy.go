// Package objectattributeadmission defines explicit trust in caller supplied attributes.
package objectattributeadmission

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
)

type Request struct{ CallerService, ResourceKey, AttributeKey string }
type Policy interface{ AuthorizeAttribute(Request) error }
type Coverage interface {
	ValidateCoverage(resourceKey string, conditions constraint.Set) error
}
type Provider struct {
	Service    string   `yaml:"service" json:"service"`
	Resource   string   `yaml:"resource" json:"resource"`
	Attributes []string `yaml:"attributes" json:"attributes"`
}

// Registry owns its normalized input and never infers wildcard trust.
type Registry struct {
	providers map[string]map[string]map[string]struct{}
}

func New(providers []Provider) (*Registry, error) {
	p := &Registry{providers: make(map[string]map[string]map[string]struct{})}
	for _, entry := range providers {
		service, resource := strings.TrimSpace(entry.Service), strings.TrimSpace(entry.Resource)
		if service == "" || resource == "" || strings.ContainsAny(service+resource, "*?") || len(entry.Attributes) == 0 {
			return nil, fmt.Errorf("provider needs exact service, resource and attribute keys")
		}
		if p.providers[resource] == nil {
			p.providers[resource] = make(map[string]map[string]struct{})
		}
		for _, key := range entry.Attributes {
			key = strings.TrimSpace(key)
			if !strings.HasPrefix(key, "object.") || len(key) <= len("object.") || strings.ContainsAny(key, "*?") {
				return nil, fmt.Errorf("invalid provider attribute: %s", key)
			}
			if p.providers[resource][key] == nil {
				p.providers[resource][key] = make(map[string]struct{})
			}
			if _, duplicate := p.providers[resource][key][service]; duplicate {
				return nil, fmt.Errorf("duplicate provider: %s %s %s", service, resource, key)
			}
			p.providers[resource][key][service] = struct{}{}
		}
	}
	return p, nil
}
func (p *Registry) AuthorizeAttribute(request Request) error {
	key, resource := strings.TrimSpace(request.AttributeKey), strings.TrimSpace(request.ResourceKey)
	if p == nil || len(p.providers[resource][key]) == 0 {
		return ErrUnsupportedAttribute{Key: key}
	}
	if _, ok := p.providers[resource][key][strings.TrimSpace(request.CallerService)]; !ok {
		return ErrUntrustedCaller{}
	}
	return nil
}
func (p *Registry) ValidateCoverage(resource string, conditions constraint.Set) error {
	for _, predicate := range conditions.AllOf {
		if p == nil || len(p.providers[resource][predicate.Key]) == 0 {
			return fmt.Errorf("resource %s attribute %s has no trusted provider", resource, predicate.Key)
		}
	}
	return nil
}
func RequireCoverage(coverage Coverage, resource string, conditions constraint.Set) error {
	if len(conditions.AllOf) == 0 {
		return nil
	}
	if coverage == nil {
		return fmt.Errorf("conditional grant for %s requires trusted attribute providers", resource)
	}
	return coverage.ValidateCoverage(resource, conditions)
}

type ErrUnsupportedAttribute struct{ Key string }

func (e ErrUnsupportedAttribute) Error() string { return "unsupported object attribute: " + e.Key }

type ErrUntrustedCaller struct{}

func (ErrUntrustedCaller) Error() string {
	return "caller is not trusted to provide this object attribute"
}

func (p *Registry) Fingerprint() string {
	data, _ := json.Marshal(p.providers)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
