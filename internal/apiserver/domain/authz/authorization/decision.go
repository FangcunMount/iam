package authorization

import (
	"sort"
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Reason string

const (
	ReasonAllowed          Reason = "allowed"
	ReasonNotMatched       Reason = "not_matched"
	ReasonAttributeMissing Reason = "attribute_missing"
)

const (
	DenyCodePolicyNotMatched = "policy_not_matched"
	DenyCodeAttributeMissing = "attribute_missing"
)

type Decision struct {
	Allowed              bool
	Reason               Reason
	DenyCode             string
	MatchedGrantID       meta.ID
	MatchedRole          string
	PolicyVersion        int64
	MissingAttributeKeys []string
	EvaluatedAt          time.Time
}

func Allow(grantID meta.ID, role string, policyVersion int64, at time.Time) Decision {
	if at.IsZero() {
		at = time.Now()
	}
	return Decision{
		Allowed: true, Reason: ReasonAllowed, MatchedGrantID: grantID,
		MatchedRole: role, PolicyVersion: policyVersion, EvaluatedAt: at,
	}
}

func Deny(policyVersion int64, missing []string, at time.Time) Decision {
	if at.IsZero() {
		at = time.Now()
	}
	missing = uniqueSorted(missing)
	if len(missing) > 0 {
		return Decision{
			Reason: ReasonAttributeMissing, DenyCode: DenyCodeAttributeMissing,
			PolicyVersion: policyVersion, MissingAttributeKeys: missing, EvaluatedAt: at,
		}
	}
	return Decision{
		Reason: ReasonNotMatched, DenyCode: DenyCodePolicyNotMatched,
		PolicyVersion: policyVersion, EvaluatedAt: at,
	}
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
