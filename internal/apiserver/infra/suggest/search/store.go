package search

import (
	"strings"
	"sync"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// Store 档案联想内存索引
type Store struct {
	trie  *Trie
	hash  *Hash
	terms map[int64]suggest.ProfileSearchTerm
	mu    sync.RWMutex
}

// Load 从档案搜索项构建 Store
func Load(terms []suggest.ProfileSearchTerm) *Store {
	s := &Store{
		trie:  NewTrie(),
		hash:  NewHash(),
		terms: make(map[int64]suggest.ProfileSearchTerm, len(terms)),
	}
	s.ImportTerms(terms)
	return s
}

// ImportTerms 合并写入档案项（全量重建或增量更新同一 profileID 时覆盖元数据）。
func (s *Store) ImportTerms(terms []suggest.ProfileSearchTerm) {
	if s == nil || len(terms) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, term := range terms {
		if term.ProfileID <= 0 || strings.TrimSpace(term.DisplayName) == "" {
			continue
		}
		s.terms[term.ProfileID] = term
		s.trie.ImportTerm(term)
		s.hash.ImportTerm(term)
	}
}

// SuggestProfile 先按关键词召回，再按 scope 过滤，最后排序截断。
func (s *Store) SuggestProfile(query suggest.Query, scope suggest.ProfileAccessScope) []suggest.ProfileSearchTerm {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchedIDs []int64
	if query.Keyword.IsDigits() {
		matchedIDs = s.hash.Match(query.Keyword.String(), query.InternalLimit)
	} else {
		matchedIDs = s.trieMatchedProfileIDs(query.Keyword.String(), query.KeyPadLen, query.InternalLimit)
	}

	visible := make([]suggest.ProfileSearchTerm, 0, min(len(matchedIDs), query.Limit))
	policy := suggest.ScopePolicy{}
	for _, id := range matchedIDs {
		term, ok := s.terms[id]
		if !ok {
			continue
		}
		if !policy.Allows(scope, term) {
			continue
		}
		visible = append(visible, term)
	}
	return suggest.RankingPolicy{}.Rank(visible, query.Limit)
}

func (s *Store) trieMatchedProfileIDs(k string, keyPadLen, internalLimit int) []int64 {
	if keyPadLen <= 0 {
		keyPadLen = suggest.DefaultKeyPadLen
	}
	rk := []rune(k)
	if len(rk) < keyPadLen {
		k = k + strings.Repeat("*", keyPadLen-len(rk))
	}
	keys := s.trie.Wildcard(k)
	var ids []int64
	seen := make(map[int64]struct{}, internalLimit)
	for _, prefixKey := range keys {
		for _, id := range s.trie.ProfileIDs(prefixKey) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			if internalLimit > 0 && len(ids) >= internalLimit {
				return ids
			}
		}
	}
	return ids
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
