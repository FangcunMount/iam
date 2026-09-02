package search

import (
	"strings"
	"sync"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	suggestmetrics "github.com/FangcunMount/iam/v3/internal/apiserver/infra/suggest/metrics"
)

type profileKeySet struct {
	trieKeys []string
	hashKeys []string
}

// Store 档案联想内存索引
type Store struct {
	trie        *Trie
	hash        *Hash
	terms       map[int64]suggest.ProfileSearchTerm
	profileKeys map[int64]profileKeySet
	mu          sync.RWMutex
}

// Load 从档案搜索项构建 Store
func Load(terms []suggest.ProfileSearchTerm) *Store {
	s := &Store{
		trie:        NewTrie(),
		hash:        NewHash(),
		terms:       make(map[int64]suggest.ProfileSearchTerm, len(terms)),
		profileKeys: make(map[int64]profileKeySet),
	}
	s.ImportTerms(terms)
	return s
}

// Len 返回当前索引中的档案条数。
func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.terms)
}

// RemoveProfile 从索引移除指定档案（含 Trie/Hash 键）。
func (s *Store) RemoveProfile(profileID int64) {
	if s == nil || profileID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeProfileLocked(profileID)
}

// ImportTerms 合并写入档案项；同一 profileID 会先撤销旧键再写入（支持增量修正）。
func (s *Store) ImportTerms(terms []suggest.ProfileSearchTerm) {
	if s == nil || len(terms) == 0 {
		return
	}
	mutations := make([]suggest.ProfileIndexMutation, 0, len(terms))
	for _, term := range terms {
		if term.ProfileID <= 0 {
			continue
		}
		if strings.TrimSpace(term.DisplayName) == "" {
			if m, err := suggest.NewProfileIndexDelete(term.ProfileID); err == nil {
				mutations = append(mutations, m)
			}
			continue
		}
		if m, err := suggest.NewProfileIndexUpsert(term); err == nil {
			mutations = append(mutations, m)
		}
	}
	s.ApplyMutations(mutations)
}

// ApplyMutations 按显式操作 upsert 或 delete。
func (s *Store) ApplyMutations(mutations []suggest.ProfileIndexMutation) {
	if s == nil || len(mutations) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profileKeys == nil {
		s.profileKeys = make(map[int64]profileKeySet)
	}
	for _, mutation := range mutations {
		switch mutation.Operation {
		case suggest.ProfileIndexDelete:
			if mutation.ProfileID > 0 {
				s.removeProfileLocked(mutation.ProfileID)
			}
		case suggest.ProfileIndexUpsert:
			term := mutation.Term
			if term.ProfileID <= 0 || strings.TrimSpace(term.DisplayName) == "" {
				continue
			}
			if prev, ok := s.profileKeys[term.ProfileID]; ok {
				s.unindexKeysLocked(prev, term.ProfileID)
			}
			tk := s.trie.ImportTerm(term)
			hk := s.hash.ImportTerm(term)
			s.terms[term.ProfileID] = term
			s.profileKeys[term.ProfileID] = profileKeySet{trieKeys: tk, hashKeys: hk}
		}
	}
}

func (s *Store) unindexKeysLocked(ks profileKeySet, profileID int64) {
	for _, k := range ks.trieKeys {
		s.trie.RemoveProfileID(k, profileID)
	}
	for _, k := range ks.hashKeys {
		s.hash.RemoveProfileID(k, profileID)
	}
}

func (s *Store) removeProfileLocked(profileID int64) {
	ks, ok := s.profileKeys[profileID]
	if ok {
		s.unindexKeysLocked(ks, profileID)
		delete(s.profileKeys, profileID)
	}
	delete(s.terms, profileID)
}

// SuggestProfile 先按 SearchMode 召回，再按 scope 过滤，最后排序截断。
func (s *Store) SuggestProfile(query suggest.Query, scope suggest.ProfileAccessScope) []suggest.ProfileSearchTerm {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	mode := query.SearchMode
	if mode == suggest.SearchModeNone {
		if query.Keyword.IsDigits() {
			mode = suggest.SearchModeExact
		} else {
			mode = suggest.SearchModePrefix
		}
	}

	var matched []suggest.RankedProfileSearchTerm
	switch mode {
	case suggest.SearchModeExact:
		matched = s.hashMatchedRanked(query)
	default:
		matched = s.trieMatchedRanked(query)
	}

	result := suggest.NewCandidateSelectionPolicy().Select(matched, scope, query)
	suggestmetrics.ObserveIndexFilter(result.MatchedCount, result.VisibleCount)
	return result.Terms
}

func (s *Store) hashMatchedRanked(q suggest.Query) []suggest.RankedProfileSearchTerm {
	ids := s.hash.Match(q.Keyword.String(), q.InternalLimit)
	out := make([]suggest.RankedProfileSearchTerm, 0, len(ids))
	for _, id := range ids {
		term, ok := s.terms[id]
		if !ok {
			continue
		}
		out = append(out, suggest.RankedProfileSearchTerm{Term: term, Kind: suggest.MatchKindExact})
	}
	return out
}

func (s *Store) trieMatchedRanked(q suggest.Query) []suggest.RankedProfileSearchTerm {
	padded := paddedTrieQueryKey(q)
	keys := s.trie.Wildcard(padded, trieWildcardCap(q))
	rawKeyword := q.Keyword.String()

	var out []suggest.RankedProfileSearchTerm
	seen := make(map[int64]struct{}, q.InternalLimit)
	for _, prefixKey := range keys {
		kind := suggest.MatchKindWildcard
		if prefixKey == padded || prefixKey == rawKeyword {
			kind = suggest.MatchKindPrefix
		}
		for _, id := range s.trie.ProfileIDs(prefixKey) {
			if _, ok := seen[id]; ok {
				continue
			}
			term, ok := s.terms[id]
			if !ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, suggest.RankedProfileSearchTerm{Term: term, Kind: kind})
			if q.InternalLimit > 0 && len(out) >= q.InternalLimit {
				return out
			}
		}
	}
	return out
}

func paddedTrieQueryKey(q suggest.Query) string {
	keyPadLen := q.KeyPadLen
	if keyPadLen <= 0 {
		keyPadLen = suggest.DefaultKeyPadLen
	}
	k := q.Keyword.String()
	rk := []rune(k)
	if len(rk) < keyPadLen {
		return k + strings.Repeat("*", keyPadLen-len(rk))
	}
	return k
}

func trieWildcardCap(q suggest.Query) int {
	maxKeys := q.WildcardKeyCap
	if maxKeys <= 0 {
		return suggest.DefaultWildcardKeyCap
	}
	return maxKeys
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
