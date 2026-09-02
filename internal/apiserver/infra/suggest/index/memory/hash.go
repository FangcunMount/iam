package memory

import (
	"strconv"
	"strings"

	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

// exactMatchIndex 支持手机号/档案 ID 的字符串精确匹配。
type exactMatchIndex struct {
	table map[string][]int64
}

func newExactMatchIndex() *exactMatchIndex {
	return &exactMatchIndex{table: make(map[string][]int64)}
}

func (h *exactMatchIndex) importProfile(p domainprofile.SuggestibleProfile) []string {
	if h == nil || p.ID() <= 0 {
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

	idKey := strconv.FormatInt(p.ID(), 10)
	addKey(idKey)
	h.table[idKey] = append(h.table[idKey], p.ID())

	for _, m := range p.Mobiles() {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		addKey(m)
		h.table[m] = append(h.table[m], p.ID())
	}
	return keys
}

func (h *exactMatchIndex) removeProfileID(key string, profileID int64) {
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

func (h *exactMatchIndex) match(keyword string, limit int) []int64 {
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
