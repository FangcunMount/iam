package suggest

import (
	"context"

	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
)

// ProfileSearchStrategy 单次联想查询中的一种搜索方式（策略模式 + 责任链）。
// 实现应按 Register 给出的顺序注册：靠前的策略优先判定 Supports。
type ProfileSearchStrategy interface {
	// Name 便于日志与排障（可选实现可用结构体常量名）。
	Name() string
	// Supports 当前关键词与可见范围是否由本策略处理。
	Supports(keyword domainsuggest.Keyword, scope domainsuggest.ProfileAccessScope) bool
	// Search 在 Supports 为 true 时执行；index/query/scope 已由应用层准备好。
	Search(ctx context.Context, index ProfileSuggestionIndex, query domainsuggest.Query, scope domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm
}

// DefaultProfileSearchStrategies 默认策略链：手机号文件且未授权 → 空；纯数字 → 哈希精确；非数字 → 前缀/拼音。
func DefaultProfileSearchStrategies() []ProfileSearchStrategy {
	return []ProfileSearchStrategy{
		mobileDeniedStrategy{},
		numericExactStrategy{},
		prefixTextStrategy{},
	}
}

type mobileDeniedStrategy struct{}

func (mobileDeniedStrategy) Name() string { return "mobile_denied" }

func (mobileDeniedStrategy) Supports(keyword domainsuggest.Keyword, scope domainsuggest.ProfileAccessScope) bool {
	return keyword.IsDigits() &&
		domainsuggest.LooksLikeMobile(keyword.String()) &&
		!scope.AllowMobileSearch
}

func (mobileDeniedStrategy) Search(context.Context, ProfileSuggestionIndex, domainsuggest.Query, domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	return nil
}

type numericExactStrategy struct{}

func (numericExactStrategy) Name() string { return "numeric_exact" }

func (numericExactStrategy) Supports(keyword domainsuggest.Keyword, _ domainsuggest.ProfileAccessScope) bool {
	return keyword.IsDigits()
}

func (numericExactStrategy) Search(_ context.Context, index ProfileSuggestionIndex, query domainsuggest.Query, scope domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	if index == nil {
		return nil
	}
	return index.SuggestProfile(query, scope)
}

type prefixTextStrategy struct{}

func (prefixTextStrategy) Name() string { return "prefix_text" }

func (prefixTextStrategy) Supports(keyword domainsuggest.Keyword, _ domainsuggest.ProfileAccessScope) bool {
	return !keyword.IsDigits()
}

func (prefixTextStrategy) Search(_ context.Context, index ProfileSuggestionIndex, query domainsuggest.Query, scope domainsuggest.ProfileAccessScope) []domainsuggest.ProfileSearchTerm {
	if index == nil {
		return nil
	}
	return index.SuggestProfile(query, scope)
}

func selectProfileSearchStrategy(strategies []ProfileSearchStrategy, keyword domainsuggest.Keyword, scope domainsuggest.ProfileAccessScope) ProfileSearchStrategy {
	for _, s := range strategies {
		if s != nil && s.Supports(keyword, scope) {
			return s
		}
	}
	return nil
}
