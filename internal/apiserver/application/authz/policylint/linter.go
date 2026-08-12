package policylint

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
)

const defaultResourceScanLimit = 10000

type PermissionFact struct {
	RoleName    string
	TenantID    string
	ResourceKey string
	Action      string
	Scope       string
}

type FactReader interface {
	ListPermissionFacts(ctx context.Context) ([]PermissionFact, error)
}

type ResourceCatalog interface {
	FindByKey(ctx context.Context, key string) (*resource.Resource, error)
	List(ctx context.Context, query resource.ResourceFilter) ([]*resource.Resource, int64, error)
}

type FindingCode string

const (
	FindingInvalidPermissionFact  FindingCode = "invalid_permission_fact"
	FindingMissingResource        FindingCode = "missing_resource"
	FindingUnsupportedAction      FindingCode = "unsupported_action"
	FindingUnsupportedScopeKind   FindingCode = "unsupported_scope_kind"
	FindingUncheckableActionRegex FindingCode = "uncheckable_action_pattern"
)

type Finding struct {
	Code        FindingCode
	RoleName    string
	TenantID    string
	ResourceKey string
	Action      string
	Scope       string
	Message     string
}

type Report struct {
	Findings []Finding
}

type Linter struct {
	facts     FactReader
	resources ResourceCatalog
}

func NewLinter(facts FactReader, resources ResourceCatalog) *Linter {
	return &Linter{facts: facts, resources: resources}
}

func (l *Linter) Lint(ctx context.Context) (Report, error) {
	if l == nil || l.facts == nil || l.resources == nil {
		return Report{}, errors.New("authorization policy linter unavailable")
	}
	facts, err := l.facts.ListPermissionFacts(ctx)
	if err != nil {
		return Report{}, err
	}
	report := Report{Findings: make([]Finding, 0)}
	for _, fact := range facts {
		report.Findings = append(report.Findings, l.lintFact(ctx, fact)...)
	}
	return report, nil
}

func (l *Linter) lintFact(ctx context.Context, fact PermissionFact) []Finding {
	scopeValue, err := scope.Parse(fact.Scope)
	if err != nil {
		return []Finding{finding(fact, FindingInvalidPermissionFact, err.Error())}
	}
	perm, err := permission.New(
		fact.RoleName,
		fact.TenantID,
		fact.ResourceKey,
		fact.Action,
		permission.WithScope(scopeValue),
	)
	if err != nil {
		return []Finding{finding(fact, FindingInvalidPermissionFact, err.Error())}
	}
	matchedResources, err := l.matchingResources(ctx, perm.ResourceKeyString())
	if err != nil {
		return []Finding{finding(fact, FindingMissingResource, err.Error())}
	}
	if len(matchedResources) == 0 {
		return []Finding{finding(fact, FindingMissingResource, "resource catalog has no matching resource")}
	}
	findings := make([]Finding, 0)
	actionPattern := perm.ActionString()
	actionRegex, regexErr := compileActionPattern(actionPattern)
	if regexErr != nil {
		findings = append(findings, finding(fact, FindingUncheckableActionRegex, regexErr.Error()))
	}
	for _, res := range matchedResources {
		if regexErr == nil && !resourceSupportsActionPattern(res, actionRegex) {
			findings = append(findings, finding(fact, FindingUnsupportedAction, fmt.Sprintf("resource %s does not support action pattern %s", res.KeyString(), actionPattern)))
		}
		if !res.AllowsScopeKind(perm.Scope.Normalized().Kind) {
			findings = append(findings, finding(fact, FindingUnsupportedScopeKind, fmt.Sprintf("resource %s does not support scope kind %s", res.KeyString(), perm.Scope.Normalized().Kind)))
		}
	}
	return findings
}

func (l *Linter) matchingResources(ctx context.Context, pattern string) ([]*resource.Resource, error) {
	if !strings.Contains(pattern, "*") {
		res, err := l.resources.FindByKey(ctx, pattern)
		if err != nil {
			return nil, err
		}
		if res == nil {
			return nil, nil
		}
		return []*resource.Resource{res}, nil
	}
	resources, _, err := l.resources.List(ctx, resource.ResourceFilter{Limit: defaultResourceScanLimit})
	if err != nil {
		return nil, err
	}
	matched := make([]*resource.Resource, 0)
	for _, res := range resources {
		if resourcePatternMatches(res.KeyString(), pattern) {
			matched = append(matched, res)
		}
	}
	return matched, nil
}

func compileActionPattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile("^(?:" + strings.TrimSpace(pattern) + ")$")
}

func resourceSupportsActionPattern(res *resource.Resource, pattern *regexp.Regexp) bool {
	if res == nil || pattern == nil {
		return false
	}
	for _, action := range res.ActionStrings() {
		if pattern.MatchString(action) {
			return true
		}
	}
	return false
}

func resourcePatternMatches(key, pattern string) bool {
	keyParts := strings.Split(strings.TrimSpace(key), ":")
	patternParts := strings.Split(strings.TrimSpace(pattern), ":")
	if len(keyParts) != 4 || len(patternParts) != 4 {
		return false
	}
	for i := range keyParts {
		if patternParts[i] == "*" {
			continue
		}
		if keyParts[i] != patternParts[i] {
			return false
		}
	}
	return true
}

func finding(fact PermissionFact, code FindingCode, message string) Finding {
	return Finding{
		Code:        code,
		RoleName:    fact.RoleName,
		TenantID:    fact.TenantID,
		ResourceKey: fact.ResourceKey,
		Action:      fact.Action,
		Scope:       fact.Scope,
		Message:     message,
	}
}
