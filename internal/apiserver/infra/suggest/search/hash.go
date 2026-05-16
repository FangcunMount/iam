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

// ImportTerm 将档案项写入精确匹配索引。
func (h *Hash) ImportTerm(term suggest.ProfileSearchTerm) {
	if h == nil {
		return
	}
	if term.ProfileID <= 0 {
		return
	}
	idKey := strconv.FormatInt(term.ProfileID, 10)
	h.table[idKey] = append(h.table[idKey], term.ProfileID)
	for _, m := range term.Mobiles {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		h.table[m] = append(h.table[m], term.ProfileID)
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
