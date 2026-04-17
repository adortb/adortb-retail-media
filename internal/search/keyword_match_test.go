package search

import (
	"testing"
)

func TestKeywordMatches_Exact(t *testing.T) {
	kw := Keyword{Text: "红色T恤", MatchType: MatchExact}
	tests := []struct {
		query string
		want  bool
	}{
		{"红色T恤", true},
		{"红色 T恤", false},
		{"红色t恤", true}, // 大小写不敏感
		{"红色T恤 size", false},
	}
	for _, tc := range tests {
		if got := kw.Matches(tc.query); got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestKeywordMatches_Phrase(t *testing.T) {
	kw := Keyword{Text: "红色T恤", MatchType: MatchPhrase}
	tests := []struct {
		query string
		want  bool
	}{
		{"红色T恤", true},
		{"夏季红色T恤特价", true},
		{"蓝色T恤", false},
	}
	for _, tc := range tests {
		if got := kw.Matches(tc.query); got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestKeywordMatches_Broad(t *testing.T) {
	kw := Keyword{Text: "red shirt", MatchType: MatchBroad}
	tests := []struct {
		query string
		want  bool
	}{
		{"red shirt", true},
		{"shirt red", true},       // 顺序无关
		{"buy red shirt now", true}, // 查询中包含所有关键词 token
		{"red", false},            // 缺少 "shirt"，不匹配
		{"blue pants", false},     // 完全不匹配
	}
	for _, tc := range tests {
		if got := kw.Matches(tc.query); got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestMatchScore(t *testing.T) {
	tests := []struct {
		kw    Keyword
		query string
		want  int
	}{
		{Keyword{"red shirt", MatchExact}, "red shirt", 3},
		{Keyword{"red shirt", MatchPhrase}, "buy red shirt now", 2},
		{Keyword{"red shirt", MatchBroad}, "shirt red", 1},
		{Keyword{"red shirt", MatchExact}, "blue shirt", 0},
	}
	for _, tc := range tests {
		if got := tc.kw.MatchScore(tc.query); got != tc.want {
			t.Errorf("MatchScore(%q) = %d, want %d", tc.query, got, tc.want)
		}
	}
}

func TestBestMatch(t *testing.T) {
	keywords := []Keyword{
		{Text: "red shirt", MatchType: MatchBroad},
		{Text: "red shirt", MatchType: MatchPhrase},
		{Text: "red shirt", MatchType: MatchExact},
	}
	_, score := BestMatch(keywords, "red shirt")
	if score != 3 {
		t.Errorf("BestMatch score = %d, want 3", score)
	}

	_, score2 := BestMatch(keywords, "buy red shirt cheap")
	if score2 != 2 {
		t.Errorf("BestMatch score = %d, want 2", score2)
	}
}
