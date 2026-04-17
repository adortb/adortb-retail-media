package search

import (
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		input    string
		tokens   []string
		negated  []string
		category string
	}{
		{
			input:  "红色T恤",
			tokens: []string{"红色t恤"},
		},
		{
			input:    "red shirt -used category:clothing",
			tokens:   []string{"red", "shirt"},
			negated:  []string{"used"},
			category: "clothing",
		},
		{
			input:  "  多余空格  关键词  ",
			tokens: []string{"多余空格", "关键词"},
		},
	}
	for _, tc := range tests {
		pq := ParseQuery(tc.input)
		if len(pq.Tokens) != len(tc.tokens) {
			t.Errorf("ParseQuery(%q) tokens=%v, want %v", tc.input, pq.Tokens, tc.tokens)
			continue
		}
		for i, tok := range tc.tokens {
			if pq.Tokens[i] != tok {
				t.Errorf("ParseQuery(%q) token[%d]=%q, want %q", tc.input, i, pq.Tokens[i], tok)
			}
		}
		if pq.Category != tc.category {
			t.Errorf("ParseQuery(%q) category=%q, want %q", tc.input, pq.Category, tc.category)
		}
	}
}

func TestQueryString(t *testing.T) {
	pq := ParseQuery("red shirt -used category:clothing")
	if got := pq.QueryString(); got != "red shirt" {
		t.Errorf("QueryString() = %q, want %q", got, "red shirt")
	}
}
