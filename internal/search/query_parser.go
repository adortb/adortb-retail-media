package search

import (
	"strings"
	"unicode"
)

// ParsedQuery 解析后的搜索查询。
type ParsedQuery struct {
	Raw      string
	Tokens   []string // 分词后的 token 列表
	Negated  []string // 以 - 开头的排除词
	Category string   // 类目限定（如 category:electronics）
}

// ParseQuery 解析搜索词，提取 token、排除词和类目限定。
func ParseQuery(raw string) ParsedQuery {
	pq := ParsedQuery{Raw: raw}
	fields := tokenize(raw)
	for _, f := range fields {
		switch {
		case strings.HasPrefix(f, "category:"):
			pq.Category = strings.TrimPrefix(f, "category:")
		case strings.HasPrefix(f, "-") && len(f) > 1:
			pq.Negated = append(pq.Negated, f[1:])
		default:
			if f != "" {
				pq.Tokens = append(pq.Tokens, f)
			}
		}
	}
	return pq
}

// QueryString 返回去除特殊指令后的纯搜索词（用于关键词匹配）。
func (pq ParsedQuery) QueryString() string {
	return strings.Join(pq.Tokens, " ")
}

func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case unicode.IsSpace(r) && !inQuote:
			if cur.Len() > 0 {
				tokens = append(tokens, strings.ToLower(cur.String()))
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, strings.ToLower(cur.String()))
	}
	return tokens
}
