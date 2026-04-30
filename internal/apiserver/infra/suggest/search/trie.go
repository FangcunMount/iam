package search

import (
	"strings"

	"github.com/mozillazg/go-pinyin"

	"github.com/FangcunMount/iam/internal/apiserver/domain/suggest"
)

const maxSearchLen = 100

// Trie 实现一个三元搜索树用于前缀/通配符查找
type Trie struct {
	root *node
}

// node 节点
type node struct {
	small *node
	equal *node
	large *node
	value Terms
	r     rune
	end   bool
}

// Terms 术语列表
type Terms []suggest.Term

// NewTrie 创建一个新的 Trie
func NewTrie() *Trie {
	return &Trie{}
}

// ImportCandidates inserts profile candidates into the prefix index.
func (t *Trie) ImportCandidates(candidates []suggest.ProfileCandidate) {
	pyArgs := pinyin.NewArgs()
	for _, candidate := range candidates {
		name := strings.TrimSpace(candidate.DisplayName)
		if name == "" {
			continue
		}
		term := candidate.Term()

		// 原始中文名
		t.Put(name, term)
		// 拼音/简拼
		py := pinyin.Pinyin(name, pyArgs)
		if len(py) == 0 {
			continue
		}
		py[0] = uniq(py[0])
		for _, a := range py[0] {
			full, abbr := a, string(a[0])
			for _, b := range py[1:] {
				full += b[0]
				abbr += string(b[0][0])
			}
			t.Put(full, term)
			t.Put(abbr, term)
		}
	}
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

// Put 插入一个术语，键为提供的字符串
func (t *Trie) Put(key string, term suggest.Term) {
	if key == "" {
		return
	}
	t.root = t.putRecursive(t.root, []rune(key), 0, term)
}

// putRecursive 递归插入术语
func (t *Trie) putRecursive(n *node, key []rune, idx int, term suggest.Term) *node {
	r := key[idx]
	if n == nil {
		n = &node{r: r}
	}
	if r < n.r {
		n.small = t.putRecursive(n.small, key, idx, term)
	} else if r > n.r {
		n.large = t.putRecursive(n.large, key, idx, term)
	} else if idx < len(key)-1 {
		n.equal = t.putRecursive(n.equal, key, idx+1, term)
	} else {
		n.end = true
		n.value = append(n.value, term)
	}
	return n
}

// Get 获取精确匹配的术语
func (t *Trie) Get(key string) interface{} {
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

// Wildcard 支持 '*' 或 '.' 通配符用于前缀匹配
func (t *Trie) Wildcard(key string) []string {
	if key == "" {
		return nil
	}
	realLen := len([]rune(strings.TrimRight(key, "*")))
	return t.wildcardRecursive(t.root, []rune(key), realLen, 0, "")
}

// wildcardRecursive 递归通配符匹配
func (t *Trie) wildcardRecursive(n *node, key []rune, realLen, idx int, prefix string) (matches []string) {
	if n == nil {
		return
	}
	if idx == len(key) {
		t.collectAll(n, prefix, &matches)
		return
	}
	r := key[idx]
	isWild := r == '*' || r == '.'
	if (isWild || r < n.r) && len(matches) < maxSearchLen {
		matches = append(matches, t.wildcardRecursive(n.small, key, realLen, idx, prefix)...)
	}
	if (isWild || r > n.r) && len(matches) < maxSearchLen {
		matches = append(matches, t.wildcardRecursive(n.large, key, realLen, idx, prefix)...)
	}
	if (isWild || r == n.r) && len(matches) < maxSearchLen {
		newPrefix := prefix + string(n.r)
		if n.end && idx >= realLen-1 {
			matches = append(matches, newPrefix)
		}
		matches = append(matches, t.wildcardRecursive(n.equal, key, realLen, idx+1, newPrefix)...)
	}
	return
}

// collectAll 收集所有终端键，最多 maxSearchLen 个
func (t *Trie) collectAll(n *node, prefix string, matches *[]string) {
	if n == nil || len(*matches) >= maxSearchLen {
		return
	}
	// explore smaller branch without adding current rune
	t.collectAll(n.small, prefix, matches)

	cur := prefix + string(n.r)
	if n.end {
		*matches = append(*matches, cur)
		if len(*matches) >= maxSearchLen {
			return
		}
	}

	t.collectAll(n.equal, cur, matches)
	t.collectAll(n.large, prefix, matches)
}
