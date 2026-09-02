package memory

import (
	"context"
	"strings"
	"sync"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	apprefresh "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/refreshindex"
	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
	domainsearch "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/search"
)

type profileKeySet struct {
	trieKeys []string
	hashKeys []string
}

// Store 档案联想内存索引。
type Store struct {
	tst         *ternarySearchTree
	hash        *exactMatchIndex
	profiles    map[int64]domainprofile.SuggestibleProfile
	profileKeys map[int64]profileKeySet
	cfg         Config
	mu          sync.RWMutex
}

// Load 从档案搜索项构建 Store。
func Load(profiles []domainprofile.SuggestibleProfile, cfg Config) *Store {
	cfg = cfg.WithDefaults()
	s := &Store{
		tst:         newTST(),
		hash:        newExactMatchIndex(),
		profiles:    make(map[int64]domainprofile.SuggestibleProfile, len(profiles)),
		profileKeys: make(map[int64]profileKeySet),
		cfg:         cfg,
	}
	s.applyUpserts(profiles)
	return s
}

// Len 返回当前索引中的档案条数。
func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.profiles)
}

// Recall 按意图召回候选，不做 scope 过滤与排序。
func (s *Store) Recall(_ context.Context, req appquery.RecallRequest) ([]domainsearch.Candidate, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch req.Intent {
	case domainsearch.IntentNumericExact:
		return s.hashMatched(req), nil
	case domainsearch.IntentTextPrefix:
		return s.trieMatched(req), nil
	default:
		return nil, nil
	}
}

func (s *Store) hashMatched(req appquery.RecallRequest) []domainsearch.Candidate {
	budget := req.CandidateBudget
	ids := s.hash.match(req.Keyword.String(), budget)
	out := make([]domainsearch.Candidate, 0, len(ids))
	for _, id := range ids {
		p, ok := s.profiles[id]
		if !ok {
			continue
		}
		out = append(out, domainsearch.Candidate{Profile: p, Strength: domainsearch.MatchExact})
	}
	return out
}

func (s *Store) trieMatched(req appquery.RecallRequest) []domainsearch.Candidate {
	padded := paddedTrieQueryKey(req.Keyword.String(), s.cfg.KeyPadLen)
	keys := s.tst.wildcard(padded, s.cfg.WildcardKeyCap)
	rawKeyword := req.Keyword.String()

	var out []domainsearch.Candidate
	seen := make(map[int64]struct{}, req.CandidateBudget)
	budget := req.CandidateBudget
	for _, prefixKey := range keys {
		strength := domainsearch.MatchExpandedPrefix
		if prefixKey == padded || prefixKey == rawKeyword {
			strength = domainsearch.MatchDirectPrefix
		}
		for _, id := range s.tst.profileIDs(prefixKey) {
			if _, ok := seen[id]; ok {
				continue
			}
			p, ok := s.profiles[id]
			if !ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, domainsearch.Candidate{Profile: p, Strength: strength})
			if budget > 0 && len(out) >= budget {
				return out
			}
		}
	}
	return out
}

func paddedTrieQueryKey(keyword string, keyPadLen int) string {
	if keyPadLen <= 0 {
		keyPadLen = DefaultKeyPadLen
	}
	rk := []rune(keyword)
	if len(rk) < keyPadLen {
		return keyword + strings.Repeat("*", keyPadLen-len(rk))
	}
	return keyword
}

// ReplaceProfiles 全量替换索引内容（由 Runtime 调用）。
func (s *Store) ReplaceProfiles(profiles []domainprofile.SuggestibleProfile) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tst = newTST()
	s.hash = newExactMatchIndex()
	s.profiles = make(map[int64]domainprofile.SuggestibleProfile, len(profiles))
	s.profileKeys = make(map[int64]profileKeySet)
	s.applyUpsertsLocked(profiles)
}

// ApplyChanges 应用投影变更。
func (s *Store) ApplyChanges(changes []apprefresh.ProjectionChange) {
	if s == nil || len(changes) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profileKeys == nil {
		s.profileKeys = make(map[int64]profileKeySet)
	}
	for _, change := range changes {
		switch change.Kind() {
		case apprefresh.ChangeDelete:
			if change.ProfileID() > 0 {
				s.removeProfileLocked(change.ProfileID())
			}
		case apprefresh.ChangeUpsert:
			p := change.Profile()
			if p.ID() <= 0 || strings.TrimSpace(p.DisplayName()) == "" {
				continue
			}
			if prev, ok := s.profileKeys[p.ID()]; ok {
				s.unindexKeysLocked(prev, p.ID())
			}
			tk := s.tst.importProfile(p)
			hk := s.hash.importProfile(p)
			s.profiles[p.ID()] = p
			s.profileKeys[p.ID()] = profileKeySet{trieKeys: tk, hashKeys: hk}
		}
	}
}

func (s *Store) applyUpserts(profiles []domainprofile.SuggestibleProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyUpsertsLocked(profiles)
}

func (s *Store) applyUpsertsLocked(profiles []domainprofile.SuggestibleProfile) {
	for _, p := range profiles {
		if p.ID() <= 0 || strings.TrimSpace(p.DisplayName()) == "" {
			continue
		}
		if prev, ok := s.profileKeys[p.ID()]; ok {
			s.unindexKeysLocked(prev, p.ID())
		}
		tk := s.tst.importProfile(p)
		hk := s.hash.importProfile(p)
		s.profiles[p.ID()] = p
		s.profileKeys[p.ID()] = profileKeySet{trieKeys: tk, hashKeys: hk}
	}
}

func (s *Store) unindexKeysLocked(ks profileKeySet, profileID int64) {
	for _, k := range ks.trieKeys {
		s.tst.removeProfileID(k, profileID)
	}
	for _, k := range ks.hashKeys {
		s.hash.removeProfileID(k, profileID)
	}
}

func (s *Store) removeProfileLocked(profileID int64) {
	ks, ok := s.profileKeys[profileID]
	if ok {
		s.unindexKeysLocked(ks, profileID)
		delete(s.profileKeys, profileID)
	}
	delete(s.profiles, profileID)
}

var _ appquery.CandidateRecaller = (*Store)(nil)
