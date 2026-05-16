package search

import (
	"strings"
	"sync"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	suggestmetrics "github.com/FangcunMount/iam/v2/internal/apiserver/infra/suggest/metrics"
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
// DisplayName 为空的项视为删除该 profile。
func (s *Store) ImportTerms(terms []suggest.ProfileSearchTerm) {
	if s == nil || len(terms) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profileKeys == nil {
		s.profileKeys = make(map[int64]profileKeySet)
	}
	for _, term := range terms {
		if term.ProfileID <= 0 {
			continue
		}
		if strings.TrimSpace(term.DisplayName) == "" {
			s.removeProfileLocked(term.ProfileID)
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
		matchedIDs = s.trieMatchedProfileIDs(query)
	}

	visible := make([]suggest.ProfileSearchTerm, 0, min(len(matchedIDs), query.Limit))
	policy := suggest.ScopePolicy{}
	compiled := suggest.CompileProfileAccessScope(scope)
	for _, id := range matchedIDs {
		term, ok := s.terms[id]
		if !ok {
			continue
		}
		if !policy.AllowsCompiled(compiled, term) {
			continue
		}
		visible = append(visible, term)
	}
	suggestmetrics.ObserveIndexFilter(len(matchedIDs), len(visible))
	return suggest.RankingPolicy{}.RankForQuery(visible, query)
}

func (s *Store) trieMatchedProfileIDs(q suggest.Query) []int64 {
	keyPadLen := q.KeyPadLen
	internalLimit := q.InternalLimit
	k := q.Keyword.String()
	if keyPadLen <= 0 {
		keyPadLen = suggest.DefaultKeyPadLen
	}
	maxKeys := q.TrieWildcardKeyCap
	if maxKeys <= 0 {
		maxKeys = suggest.DefaultTrieWildcardKeyCap
	}
	rk := []rune(k)
	if len(rk) < keyPadLen {
		k = k + strings.Repeat("*", keyPadLen-len(rk))
	}
	keys := s.trie.Wildcard(k, maxKeys)
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
