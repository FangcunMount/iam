package search

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

// Store 存储器
type Store struct {
	trie  *Trie
	table *Hash
	mu    sync.RWMutex
}

var active atomic.Value // 当前活跃的存储器

// Load 从原始行构建 Store
func Load(lines []string) *Store {
	t := NewTrie()
	h := NewHash()
	t.ImportLines(lines)
	h.ImportLines(lines)
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

// ImportLines 追加新数据到现有存储器（受锁保护）
func (s *Store) ImportLines(lines []string) {
	if s == nil || len(lines) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trie.ImportLines(lines)
	s.table.ImportLines(lines)
}

// Suggest 返回有序且去重的术语
func (s *Store) Suggest(keyword string, max int, pad int) []suggest.Term {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if max <= 0 {
		max = 20
	}

	// 数字走 Hash
	if isDigits(keyword) {
		terms := Terms(s.table.Search(keyword))
		terms = RemoveDuplicate(terms)
		sort.Sort(terms)
		if len(terms) > max {
			terms = terms[:max]
		}
		return terms
	}
	// 前缀通配
	k := keyword
	if len([]rune(k)) < pad {
		k = k + strings.Repeat("*", pad-len([]rune(k)))
	}
	keys := s.trie.Wildcard(k)
	var out Terms
	for _, key := range keys {
		if v := s.trie.Get(key); v != nil {
			out = append(out, v.(Terms)...)
		}
	}
	out = RemoveDuplicate(out)
	sort.Sort(out)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// isDigits 判断是否为数字
func isDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}
