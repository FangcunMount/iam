package search

import (
	"strconv"
	"strings"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

// Hash 支持手机号/ID 精确匹配
type Hash struct {
	table map[int64][]suggest.Term
}

// NewHash constructs a Hash store.
func NewHash() *Hash {
	return &Hash{table: make(map[int64][]suggest.Term)}
}

// ImportLines loads name|id|mobiles|disease|weight formatted rows.
func (h *Hash) ImportLines(lines []string) {
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		idStr := strings.TrimSpace(parts[1])
		mobileStr := strings.TrimSpace(parts[2])
		_ = strings.TrimSpace(parts[3]) // disease field ignored
		weight, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
		id, _ := strconv.ParseInt(idStr, 10, 64)
		mobiles := strings.Split(mobileStr, ",")
		term := suggest.Term{Name: name, ID: id, Mobile: mobileStr, Weight: weight}
		if id != 0 {
			h.table[id] = append(h.table[id], term)
		}
		for _, m := range mobiles {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			mid, err := strconv.ParseInt(m, 10, 64)
			if err != nil {
				continue
			}
			h.table[mid] = append(h.table[mid], term)
		}
	}
}

// Search returns entries for an exact numeric key.
func (h *Hash) Search(key string) []suggest.Term {
	k, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return nil
	}
	return h.table[k]
}
