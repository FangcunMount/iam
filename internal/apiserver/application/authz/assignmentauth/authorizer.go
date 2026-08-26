package assignmentauth

import (
	"fmt"
	"sort"
	"strings"
)

type Operation string

const (
	OperationGrant   Operation = "grant"
	OperationRevoke  Operation = "revoke"
	OperationReplace Operation = "replace"
)

type Request struct {
	CallerService  string
	Operation      Operation
	Subject        string
	Domain         string
	RoleName       string
	DelegatedActor string
}

type ReplacementRequest struct {
	CallerService  string
	Subject        string
	Domain         string
	RoleNames      []string
	DelegatedActor string
}

type ServiceConstraint struct {
	AllowAll                     bool     `yaml:"allow_all"`
	Domains                      []string `yaml:"domains"`
	SubjectTypes                 []string `yaml:"subject_types"`
	Roles                        []string `yaml:"roles"`
	RequireDelegatedActorOnGrant bool     `yaml:"require_delegated_actor_on_grant"`
}

type Config struct {
	DefaultPolicy string                       `yaml:"default_policy"`
	Services      map[string]ServiceConstraint `yaml:"services"`
}

type Authorizer interface {
	AuthorizeAssignment(Request) error
	AuthorizeReplacement(ReplacementRequest) ([]string, error)
}

type DeniedError struct {
	Reason string
}

func (e *DeniedError) Error() string {
	return "assignment request denied: " + e.Reason
}

type ruleAuthorizer struct {
	config Config
}

func New(config Config) (Authorizer, error) {
	if err := Validate(config); err != nil {
		return nil, err
	}
	return &ruleAuthorizer{config: config}, nil
}

func Validate(config Config) error {
	if !strings.EqualFold(strings.TrimSpace(config.DefaultPolicy), "deny") {
		return fmt.Errorf("assignment constraints default_policy must be deny")
	}
	if len(config.Services) == 0 {
		return fmt.Errorf("assignment constraints require at least one service")
	}
	for serviceName, constraint := range config.Services {
		serviceName = strings.TrimSpace(serviceName)
		if serviceName == "" {
			return fmt.Errorf("assignment constraint service name is required")
		}
		if constraint.AllowAll {
			if serviceName != "admin" {
				return fmt.Errorf("assignment constraint allow_all is only valid for admin")
			}
			continue
		}
		if len(normalizeSet(constraint.Domains)) == 0 ||
			len(normalizeSet(constraint.SubjectTypes)) == 0 ||
			len(normalizeSet(constraint.Roles)) == 0 {
			return fmt.Errorf("assignment constraint for %s requires domains, subject_types, and roles", serviceName)
		}
	}
	return nil
}

func (a *ruleAuthorizer) AuthorizeAssignment(request Request) error {
	serviceName := strings.TrimSpace(request.CallerService)
	constraint, ok := a.config.Services[serviceName]
	if !ok {
		return &DeniedError{Reason: "service_not_configured"}
	}
	if constraint.AllowAll {
		return nil
	}
	if !contains(constraint.Domains, request.Domain) {
		return &DeniedError{Reason: "domain_not_allowed"}
	}
	parts := strings.SplitN(strings.TrimSpace(request.Subject), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !contains(constraint.SubjectTypes, parts[0]) {
		return &DeniedError{Reason: "subject_not_allowed"}
	}
	if !contains(constraint.Roles, request.RoleName) {
		return &DeniedError{Reason: "role_not_allowed"}
	}
	if request.Operation == OperationGrant && constraint.RequireDelegatedActorOnGrant {
		actor := strings.SplitN(strings.TrimSpace(request.DelegatedActor), ":", 2)
		if len(actor) != 2 || actor[0] != "user" || actor[1] == "" {
			return &DeniedError{Reason: "delegated_actor_required"}
		}
	}
	return nil
}

func (a *ruleAuthorizer) AuthorizeReplacement(request ReplacementRequest) ([]string, error) {
	serviceName := strings.TrimSpace(request.CallerService)
	constraint, ok := a.config.Services[serviceName]
	if !ok {
		return nil, &DeniedError{Reason: "service_not_configured"}
	}
	if constraint.AllowAll {
		return nil, &DeniedError{Reason: "replacement_requires_explicit_managed_roles"}
	}
	if !contains(constraint.Domains, request.Domain) {
		return nil, &DeniedError{Reason: "domain_not_allowed"}
	}
	parts := strings.SplitN(strings.TrimSpace(request.Subject), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !contains(constraint.SubjectTypes, parts[0]) {
		return nil, &DeniedError{Reason: "subject_not_allowed"}
	}
	managedRoles := sortedSet(constraint.Roles)
	managedSet := normalizeSet(managedRoles)
	for _, roleName := range request.RoleNames {
		if _, ok := managedSet[strings.TrimSpace(roleName)]; !ok {
			return nil, &DeniedError{Reason: "role_not_allowed"}
		}
	}
	if constraint.RequireDelegatedActorOnGrant {
		actor := strings.SplitN(strings.TrimSpace(request.DelegatedActor), ":", 2)
		validUser := len(actor) == 2 && actor[0] == "user" && actor[1] != ""
		validCallingService := len(actor) == 2 && actor[0] == "service" && actor[1] == serviceName
		if !validUser && !validCallingService {
			return nil, &DeniedError{Reason: "delegated_actor_required"}
		}
	}
	return managedRoles, nil
}

func contains(values []string, wanted string) bool {
	_, ok := normalizeSet(values)[strings.TrimSpace(wanted)]
	return ok
}

func normalizeSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func sortedSet(values []string) []string {
	set := normalizeSet(values)
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
