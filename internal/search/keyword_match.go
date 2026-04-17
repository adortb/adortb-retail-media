// Package search 实现搜索词解析、关键词匹配及有机+广告混合排序。
package search

import (
	"strings"
	"unicode"
)

// MatchType 关键词匹配类型。
type MatchType string

const (
	MatchExact  MatchType = "exact"
	MatchPhrase MatchType = "phrase"
	MatchBroad  MatchType = "broad"
)

// Keyword 带匹配类型的关键词。
type Keyword struct {
	Text      string
	MatchType MatchType
}

// Matches 判断搜索词 query 是否与关键词匹配。
// 优先级: exact > phrase > broad。
func (k Keyword) Matches(query string) bool {
	q := normalize(query)
	kw := normalize(k.Text)
	switch k.MatchType {
	case MatchExact:
		return q == kw
	case MatchPhrase:
		return strings.Contains(q, kw)
	case MatchBroad:
		return broadMatch(q, kw)
	}
	return false
}

// MatchScore 返回匹配分数（exact=3, phrase=2, broad=1, 不匹配=0）。
func (k Keyword) MatchScore(query string) int {
	if !k.Matches(query) {
		return 0
	}
	switch k.MatchType {
	case MatchExact:
		return 3
	case MatchPhrase:
		return 2
	case MatchBroad:
		return 1
	}
	return 0
}

// BestMatch 从关键词列表中找到得分最高的匹配。
func BestMatch(keywords []Keyword, query string) (Keyword, int) {
	best := Keyword{}
	bestScore := 0
	for _, kw := range keywords {
		score := kw.MatchScore(query)
		if score > bestScore {
			bestScore = score
			best = kw
		}
	}
	return best, bestScore
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prev := ' '
	for _, r := range s {
		if unicode.IsSpace(r) {
			if prev != ' ' {
				b.WriteRune(' ')
			}
			prev = ' '
		} else {
			b.WriteRune(r)
			prev = r
		}
	}
	return strings.TrimSpace(b.String())
}

// broadMatch 检查关键词的所有单词是否都出现在查询词中（顺序无关）。
// 广泛匹配要求查询必须覆盖关键词所有 token，多余词允许存在。
func broadMatch(query, keyword string) bool {
	queryTokens := strings.Fields(query)
	kwTokens := strings.Fields(keyword)
	if len(kwTokens) == 0 {
		return false
	}
	querySet := make(map[string]struct{}, len(queryTokens))
	for _, t := range queryTokens {
		querySet[t] = struct{}{}
	}
	for _, t := range kwTokens {
		if _, ok := querySet[t]; !ok {
			return false
		}
	}
	return true
}
