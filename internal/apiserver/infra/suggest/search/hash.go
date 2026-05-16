package search

import (
	"strconv"
	"strings"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// Hash 支持手机号/档案 ID 的字符串精确匹配 -> profileID 列表
type Hash struct {
	table map[string][]int64
}

// NewHash constructs a Hash store.
func NewHash() *Hash {
	return &Hash{table: make(map[string][]int64)}
}

// ImportTerm 将档案项写入精确匹配索引，返回写入的键（profile id 字符串与手机号，去重）。
func (h *Hash) ImportTerm(term suggest.ProfileSearchTerm) []string {
	if h == nil || term.ProfileID <= 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var keys []string
	addKey := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		keys = append(keys, s)
	}

	idKey := strconv.FormatInt(term.ProfileID, 10)
	addKey(idKey)
	h.table[idKey] = append(h.table[idKey], term.ProfileID)

	for _, m := range term.Mobiles {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		addKey(m)
		h.table[m] = append(h.table[m], term.ProfileID)
	}
	return keys
}

// RemoveProfileID 从精确键对应的列表中移除 profileID（列表空则删除键）。
func (h *Hash) RemoveProfileID(key string, profileID int64) {
	if h == nil {
		return
	}
	key = strings.TrimSpace(key)
	ids, ok := h.table[key]
	if !ok {
		return
	}
	next := stripProfileID(ids, profileID)
	if len(next) == 0 {
		delete(h.table, key)
	} else {
		h.table[key] = next
	}
}

// Match 返回关键词对应的 profileID 列表（可截断）。
func (h *Hash) Match(keyword string, limit int) []int64 {
	if h == nil {
		return nil
	}
	key := strings.TrimSpace(keyword)
	ids := h.table[key]
	if limit > 0 && len(ids) > limit {
		return ids[:limit]
	}
	return ids
}
