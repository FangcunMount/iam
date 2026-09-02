package suggest

// KeywordKind 表达关键词形态分类。
type KeywordKind int

const (
	KeywordKindEmpty KeywordKind = iota
	KeywordKindText
	KeywordKindNumeric
	KeywordKindMobileShaped
)

// Kind 返回关键词形态。
func (k Keyword) Kind() KeywordKind {
	if k.value == "" {
		return KeywordKindEmpty
	}
	if !k.IsDigits() {
		return KeywordKindText
	}
	if LooksLikeMobile(k.value) {
		return KeywordKindMobileShaped
	}
	return KeywordKindNumeric
}

// IsMobileShaped 是否为手机号形态（7–15 位纯数字）。
func (k Keyword) IsMobileShaped() bool {
	return k.Kind() == KeywordKindMobileShaped
}

// SearchMode 表达索引召回方式。
type SearchMode int

const (
	SearchModeNone SearchMode = iota
	SearchModeExact
	SearchModePrefix
)

type searchDecisionKind int

const (
	searchDecisionDenied searchDecisionKind = iota
	searchDecisionExact
	searchDecisionPrefix
)

// SearchDecision 是封闭的搜索决策结果。
type SearchDecision struct {
	kind searchDecisionKind
}

// SearchPolicy 根据关键词与可见范围决定搜索方式。
type SearchPolicy struct{}

// Decide 返回 mobile_denied、numeric_exact 或 prefix_text 对应的决策。
func (SearchPolicy) Decide(keyword Keyword, scope ProfileAccessScope) SearchDecision {
	switch keyword.Kind() {
	case KeywordKindEmpty:
		return SearchDecision{kind: searchDecisionDenied}
	case KeywordKindMobileShaped:
		if !scope.AllowMobileSearch {
			return SearchDecision{kind: searchDecisionDenied}
		}
		return SearchDecision{kind: searchDecisionExact}
	case KeywordKindNumeric:
		return SearchDecision{kind: searchDecisionExact}
	default:
		return SearchDecision{kind: searchDecisionPrefix}
	}
}

// Allowed 是否允许查询索引。
func (d SearchDecision) Allowed() bool {
	return d.kind != searchDecisionDenied
}

// MetricName 返回 Prometheus 策略标签。
func (d SearchDecision) MetricName() string {
	switch d.kind {
	case searchDecisionDenied:
		return "mobile_denied"
	case searchDecisionExact:
		return "numeric_exact"
	default:
		return "prefix_text"
	}
}

// Mode 返回索引召回模式；denied 时为 SearchModeNone。
func (d SearchDecision) Mode() SearchMode {
	switch d.kind {
	case searchDecisionExact:
		return SearchModeExact
	case searchDecisionPrefix:
		return SearchModePrefix
	default:
		return SearchModeNone
	}
}

// MobileShaped 是否计入手机号形态指标。
func (d SearchDecision) MobileShaped(keyword Keyword) bool {
	return keyword.IsMobileShaped()
}
