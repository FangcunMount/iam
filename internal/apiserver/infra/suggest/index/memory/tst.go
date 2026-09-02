package memory

import (
	"strings"

	"github.com/mozillazg/go-pinyin"

	domainprofile "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

// ternarySearchTree 实现三元搜索树用于前缀/通配符查找。
type ternarySearchTree struct {
	root *tstNode
}

type tstNode struct {
	small *tstNode
	equal *tstNode
	large *tstNode
	value []int64
	r     rune
	end   bool
}

func newTST() *ternarySearchTree {
	return &ternarySearchTree{}
}

func (t *ternarySearchTree) importProfile(p domainprofile.SuggestibleProfile) []string {
	if t == nil {
		return nil
	}
	name := strings.TrimSpace(p.DisplayName())
	if name == "" || p.ID() <= 0 {
		return nil
	}
	pid := p.ID()
	pyArgs := pinyin.NewArgs()
	seen := make(map[string]struct{})
	var keys []string
	addKey := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	put := func(key string) {
		addKey(key)
		t.put(key, pid)
	}

	put(name)
	py := pinyin.Pinyin(name, pyArgs)
	if len(py) == 0 {
		return keys
	}
	py[0] = uniq(py[0])
	for _, a := range py[0] {
		full, abbr := a, string(a[0])
		for _, b := range py[1:] {
			full += b[0]
			abbr += string(b[0][0])
		}
		put(full)
		put(abbr)
	}
	return keys
}

func uniq(list []string) []string {
	var out []string
	for _, s := range list {
		exists := false
		for _, v := range out {
			if s == v {
				exists = true
				break
			}
		}
		if !exists {
			out = append(out, s)
		}
	}
	return out
}

func (t *ternarySearchTree) put(key string, profileID int64) {
	if key == "" || profileID <= 0 {
		return
	}
	t.root = t.putRecursive(t.root, []rune(key), 0, profileID)
}

func (t *ternarySearchTree) putRecursive(n *tstNode, key []rune, idx int, profileID int64) *tstNode {
	r := key[idx]
	if n == nil {
		n = &tstNode{r: r}
	}
	if r < n.r {
		n.small = t.putRecursive(n.small, key, idx, profileID)
	} else if r > n.r {
		n.large = t.putRecursive(n.large, key, idx, profileID)
	} else if idx < len(key)-1 {
		n.equal = t.putRecursive(n.equal, key, idx+1, profileID)
	} else {
		n.end = true
		n.value = append(n.value, profileID)
	}
	return n
}

func (t *ternarySearchTree) profileIDs(key string) []int64 {
	n := t.root
	rkey := []rune(key)
	for i, r := range rkey {
		for n != nil {
			if r < n.r {
				n = n.small
			} else if r > n.r {
				n = n.large
			} else {
				if i == len(rkey)-1 && n.end {
					return n.value
				}
				n = n.equal
				break
			}
		}
		if n == nil {
			return nil
		}
	}
	return nil
}

func (t *ternarySearchTree) removeProfileID(key string, profileID int64) {
	if t == nil || key == "" || profileID <= 0 {
		return
	}
	n := t.terminalNode(key)
	if n == nil || !n.end {
		return
	}
	n.value = stripProfileID(n.value, profileID)
}

func (t *ternarySearchTree) terminalNode(key string) *tstNode {
	n := t.root
	rkey := []rune(key)
	for i, r := range rkey {
		for n != nil {
			if r < n.r {
				n = n.small
			} else if r > n.r {
				n = n.large
			} else {
				if i == len(rkey)-1 {
					if n.end {
						return n
					}
					return nil
				}
				n = n.equal
				break
			}
		}
		if n == nil {
			return nil
		}
	}
	return nil
}

func stripProfileID(ids []int64, pid int64) []int64 {
	if len(ids) == 0 {
		return ids
	}
	out := ids[:0]
	for _, id := range ids {
		if id != pid {
			out = append(out, id)
		}
	}
	return out
}

func (t *ternarySearchTree) wildcard(key string, maxKeys int) []string {
	if key == "" || t == nil {
		return nil
	}
	if maxKeys <= 0 {
		maxKeys = DefaultWildcardKeyCap
	}
	realLen := len([]rune(strings.TrimRight(key, "*")))
	return t.wildcardRecursive(t.root, []rune(key), realLen, 0, "", maxKeys)
}

func (t *ternarySearchTree) wildcardRecursive(n *tstNode, key []rune, realLen, idx int, prefix string, maxKeys int) (matches []string) {
	if n == nil {
		return
	}
	if idx == len(key) {
		t.collectAll(n, prefix, &matches, maxKeys)
		return
	}
	r := key[idx]
	isWild := r == '*' || r == '.'
	if (isWild || r < n.r) && len(matches) < maxKeys {
		matches = append(matches, t.wildcardRecursive(n.small, key, realLen, idx, prefix, maxKeys)...)
	}
	if (isWild || r > n.r) && len(matches) < maxKeys {
		matches = append(matches, t.wildcardRecursive(n.large, key, realLen, idx, prefix, maxKeys)...)
	}
	if (isWild || r == n.r) && len(matches) < maxKeys {
		newPrefix := prefix + string(n.r)
		if n.end && idx >= realLen-1 {
			matches = append(matches, newPrefix)
		}
		matches = append(matches, t.wildcardRecursive(n.equal, key, realLen, idx+1, newPrefix, maxKeys)...)
	}
	return
}

func (t *ternarySearchTree) collectAll(n *tstNode, prefix string, matches *[]string, maxKeys int) {
	if n == nil || len(*matches) >= maxKeys {
		return
	}
	t.collectAll(n.small, prefix, matches, maxKeys)

	cur := prefix + string(n.r)
	if n.end {
		*matches = append(*matches, cur)
		if len(*matches) >= maxKeys {
			return
		}
	}

	t.collectAll(n.equal, cur, matches, maxKeys)
	t.collectAll(n.large, prefix, matches, maxKeys)
}
