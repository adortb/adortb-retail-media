package search

import (
	"testing"

	"github.com/adortb/adortb-retail-media/internal/catalog"
)

func TestQualityScore(t *testing.T) {
	tests := []struct {
		ctr     float64
		rating  float64
		price   float64
		avgCat  float64
		wantGT  float64 // 结果应大于此值
	}{
		{0.05, 5.0, 50.0, 100.0, 0.1},  // 高评分+低价高 QS
		{0.01, 1.0, 200.0, 50.0, 0.001}, // 低评分+高价低 QS
		{0.0, 4.0, 80.0, 100.0, 0.001},  // CTR=0 使用默认 0.01
	}
	for _, tc := range tests {
		got := QualityScore(tc.ctr, tc.rating, tc.price, tc.avgCat)
		if got <= tc.wantGT {
			t.Errorf("QualityScore(%v,%v,%v,%v) = %v, want > %v",
				tc.ctr, tc.rating, tc.price, tc.avgCat, got, tc.wantGT)
		}
	}
}

func TestRankAds_Order(t *testing.T) {
	products := []*catalog.Product{
		{SKU: "A", Price: 100, Rating: 4.5},
		{SKU: "B", Price: 50, Rating: 3.0},
		{SKU: "C", Price: 200, Rating: 5.0},
	}
	candidates := []AdCandidate{
		{Product: products[0], Bid: 1.0, CTRHistory: 0.05, AvgCatPrice: 100},
		{Product: products[1], Bid: 2.0, CTRHistory: 0.02, AvgCatPrice: 100},
		{Product: products[2], Bid: 0.5, CTRHistory: 0.1, AvgCatPrice: 100},
	}
	ranked := RankAds(candidates, 2)
	if len(ranked) != 2 {
		t.Fatalf("RankAds returned %d, want 2", len(ranked))
	}
	if ranked[0].AdScore < ranked[1].AdScore {
		t.Error("ranked items not in descending order")
	}
}

func TestRankAds_EmptySlots(t *testing.T) {
	result := RankAds(nil, 3)
	if result != nil {
		t.Error("expected nil for empty candidates")
	}
	result2 := RankAds([]AdCandidate{{Product: &catalog.Product{}}}, 0)
	if result2 != nil {
		t.Error("expected nil for 0 slots")
	}
}
