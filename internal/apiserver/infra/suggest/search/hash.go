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

// ImportCandidates loads profile candidates into the exact numeric index.
func (h *Hash) ImportCandidates(candidates []suggest.ProfileCandidate) {
	for _, candidate := range candidates {
		term := candidate.Term()
		if candidate.ProfileID != 0 {
			h.table[candidate.ProfileID] = append(h.table[candidate.ProfileID], term)
		}
		for _, m := range candidate.Mobiles {
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
