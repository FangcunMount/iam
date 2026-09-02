package search

import (
	"strings"
	"unicode"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

// Keyword 表达一次档案联想查询的关键字。
type Keyword struct {
	value string
}

// NewKeyword 构造并 trim 关键词。
func NewKeyword(value string) Keyword {
	return Keyword{value: strings.TrimSpace(value)}
}

func (k Keyword) String() string { return k.value }

func (k Keyword) IsDigits() bool {
	if k.value == "" {
		return false
	}
	for _, r := range k.value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// IsMobileShaped 是否为手机号形态（7–15 位纯数字）。
func (k Keyword) IsMobileShaped() bool {
	if k.value == "" || !k.IsDigits() {
		return false
	}
	return profile.LooksLikeMobile(k.value)
}

// Intent 表达索引召回意图。
type Intent uint8

const (
	IntentNone Intent = iota
	IntentNumericExact
	IntentTextPrefix
)

// DenialReason 表达查询准入拒绝原因。
type DenialReason uint8

const (
	DenialNone DenialReason = iota
	DenialEmptyKeyword
	DenialMobileSearchForbidden
)

// DecisionKind 供 metrics adapter 映射策略标签。
type DecisionKind uint8

const (
	DecisionDenied DecisionKind = iota
	DecisionNumericExact
	DecisionPrefixText
)

// Decision 是封闭的查询准入决策。
type Decision struct {
	intent       Intent
	denialReason DenialReason
}

func (d Decision) Allowed() bool              { return d.denialReason == DenialNone && d.intent != IntentNone }
func (d Decision) Intent() Intent             { return d.intent }
func (d Decision) DenialReason() DenialReason { return d.denialReason }

// Kind 返回 metrics 映射用决策种类。
func (d Decision) Kind() DecisionKind {
	if d.denialReason != DenialNone {
		return DecisionDenied
	}
	switch d.intent {
	case IntentNumericExact:
		return DecisionNumericExact
	default:
		return DecisionPrefixText
	}
}

// MobileShaped 是否计入手机号形态指标。
func (d Decision) MobileShaped(keyword Keyword) bool {
	return keyword.IsMobileShaped()
}

// AdmissionPolicy 根据关键词与可见范围决定查询准入。
type AdmissionPolicy struct{}

// Decide 返回准入决策。
func (AdmissionPolicy) Decide(keyword Keyword, scope visibility.Scope) Decision {
	if keyword.String() == "" {
		return Decision{denialReason: DenialEmptyKeyword}
	}
	if keyword.IsMobileShaped() && !scope.AllowsMobileSearch() {
		return Decision{denialReason: DenialMobileSearchForbidden}
	}
	if keyword.IsDigits() {
		return Decision{intent: IntentNumericExact}
	}
	return Decision{intent: IntentTextPrefix}
}
