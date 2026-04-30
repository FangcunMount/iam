package search

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

// Store 存储器
type Store struct {
	trie  *Trie
	table *Hash
	mu    sync.RWMutex
}

var active atomic.Value // 当前活跃的存储器

// Load 从档案候选构建 Store
func Load(candidates []suggest.ProfileCandidate) *Store {
	t := NewTrie()
	h := NewHash()
	t.ImportCandidates(candidates)
	h.ImportCandidates(candidates)
	return &Store{trie: t, table: h}
}

// Swap 原子替换当前活跃的存储器
func Swap(s *Store) { active.Store(s) }

// Current 返回当前活跃的存储器
func Current() *Store {
	if v := active.Load(); v != nil {
		return v.(*Store)
	}
	return nil
}

// ImportCandidates 追加新数据到现有存储器（受锁保护）
func (s *Store) ImportCandidates(candidates []suggest.ProfileCandidate) {
	if s == nil || len(candidates) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trie.ImportCandidates(candidates)
	s.table.ImportCandidates(candidates)
}

// Suggest 返回有序且去重的术语
func (s *Store) Suggest(query suggest.Query) []suggest.Term {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 数字走 Hash
	if query.Keyword.IsDigits() {
		return suggest.RankingPolicy{}.Rank(s.table.Search(query.Keyword.String()), query.Limit)
	}
	// 前缀通配
	k := query.Keyword.String()
	if len([]rune(k)) < query.KeyPadLen {
		k = k + strings.Repeat("*", query.KeyPadLen-len([]rune(k)))
	}
	keys := s.trie.Wildcard(k)
	var out Terms
	for _, key := range keys {
		if v := s.trie.Get(key); v != nil {
			out = append(out, v.(Terms)...)
		}
	}
	return suggest.RankingPolicy{}.Rank(out, query.Limit)
}
