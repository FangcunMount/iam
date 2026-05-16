package search

import (
	"strings"

	"github.com/mozillazg/go-pinyin"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
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
	value []int64
	r     rune
	end   bool
}

// NewTrie 创建一个新的 Trie
func NewTrie() *Trie {
	return &Trie{}
}

// ImportTerm inserts profile search keys for one profile.
func (t *Trie) ImportTerm(term suggest.ProfileSearchTerm) {
	if t == nil {
		return
	}
	name := strings.TrimSpace(term.DisplayName)
	if name == "" || term.ProfileID <= 0 {
		return
	}
	pid := term.ProfileID
	pyArgs := pinyin.NewArgs()

	// 原始中文名
	t.Put(name, pid)
	// 拼音/简拼
	py := pinyin.Pinyin(name, pyArgs)
	if len(py) == 0 {
		return
	}
	py[0] = uniq(py[0])
	for _, a := range py[0] {
		full, abbr := a, string(a[0])
		for _, b := range py[1:] {
			full += b[0]
			abbr += string(b[0][0])
		}
		t.Put(full, pid)
		t.Put(abbr, pid)
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
