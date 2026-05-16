package search

import (
	"strings"

	"github.com/mozillazg/go-pinyin"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

// Trie 实现一个三元搜索树用于前缀/通配符查找
type Trie struct {
	root *node
}

// node 节点
type node struct {
	small *node
	equal *node
	large *node
	value []int64
	r     rune
	end   bool
}

// NewTrie 创建一个新的 Trie
func NewTrie() *Trie {
	return &Trie{}
}

// ImportTerm inserts profile search keys for one profile and返回本 term 写入的键（去重）。
func (t *Trie) ImportTerm(term suggest.ProfileSearchTerm) []string {
	if t == nil {
		return nil
	}
	name := strings.TrimSpace(term.DisplayName)
	if name == "" || term.ProfileID <= 0 {
		return nil
	}
	pid := term.ProfileID
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
		t.Put(key, pid)
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

// uniq 去重
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

// Put 插入 profileID，键为提供的字符串
func (t *Trie) Put(key string, profileID int64) {
	if key == "" || profileID <= 0 {
		return
	}
	t.root = t.putRecursive(t.root, []rune(key), 0, profileID)
}

func (t *Trie) putRecursive(n *node, key []rune, idx int, profileID int64) *node {
	r := key[idx]
	if n == nil {
		n = &node{r: r}
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

// ProfileIDs 获取精确匹配键下的 profileID 列表
func (t *Trie) ProfileIDs(key string) []int64 {
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

// RemoveProfileID 从精确键的终端列表中移除 profileID（用于增量修正）。
func (t *Trie) RemoveProfileID(key string, profileID int64) {
	if t == nil || key == "" || profileID <= 0 {
		return
	}
	n := t.terminalNode(key)
	if n == nil || !n.end {
		return
	}
	n.value = stripProfileID(n.value, profileID)
}

func (t *Trie) terminalNode(key string) *node {
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

// Wildcard 支持 '*' 或 '.' 通配符用于前缀匹配；maxKeys<=0 时使用 domain 默认值。
func (t *Trie) Wildcard(key string, maxKeys int) []string {
	if key == "" || t == nil {
		return nil
	}
	if maxKeys <= 0 {
		maxKeys = suggest.DefaultTrieWildcardKeyCap
	}
	realLen := len([]rune(strings.TrimRight(key, "*")))
	return t.wildcardRecursive(t.root, []rune(key), realLen, 0, "", maxKeys)
}

// wildcardRecursive 递归通配符匹配
func (t *Trie) wildcardRecursive(n *node, key []rune, realLen, idx int, prefix string, maxKeys int) (matches []string) {
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

// collectAll 收集所有终端键，最多 maxKeys 个
func (t *Trie) collectAll(n *node, prefix string, matches *[]string, maxKeys int) {
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
