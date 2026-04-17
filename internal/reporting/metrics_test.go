package reporting

import (
	"math"
	"testing"
)

func TestACOS(t *testing.T) {
	tests := []struct {
		spend, sales float64
		want         float64
	}{
		{10, 100, 0.1},
		{50, 0, 0}, // 零除保护
		{0, 100, 0},
	}
	for _, tc := range tests {
		got := ACOS(tc.spend, tc.sales)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("ACOS(%v,%v) = %v, want %v", tc.spend, tc.sales, got, tc.want)
		}
	}
}

func TestROAS(t *testing.T) {
	if got := ROAS(100, 10); math.Abs(got-10.0) > 0.001 {
		t.Errorf("ROAS = %v, want 10.0", got)
	}
	if got := ROAS(100, 0); got != 0 {
		t.Errorf("ROAS zero spend = %v, want 0", got)
	}
}

func TestTACoS(t *testing.T) {
	if got := TACoS(20, 200); math.Abs(got-0.1) > 0.001 {
		t.Errorf("TACoS = %v, want 0.1", got)
	}
	if got := TACoS(20, 0); got != 0 {
		t.Errorf("TACoS zero sales = %v, want 0", got)
	}
}

func TestMetrics_Computed(t *testing.T) {
	m := &Metrics{
		Impressions: 1000,
		Clicks:      50,
		Spend:       100,
		Purchases:   5,
		Sales:       500,
	}
	m.CTR = safeDiv(float64(m.Clicks), float64(m.Impressions))
	m.CVR = safeDiv(float64(m.Purchases), float64(m.Clicks))
	m.ACOS = ACOS(m.Spend, m.Sales)
	m.ROAS = ROAS(m.Sales, m.Spend)
	m.AvgCPC = safeDiv(m.Spend, float64(m.Clicks))

	if math.Abs(m.CTR-0.05) > 0.001 {
		t.Errorf("CTR = %v, want 0.05", m.CTR)
	}
	if math.Abs(m.CVR-0.1) > 0.001 {
		t.Errorf("CVR = %v, want 0.1", m.CVR)
	}
	if math.Abs(m.ACOS-0.2) > 0.001 {
		t.Errorf("ACOS = %v, want 0.2", m.ACOS)
	}
	if math.Abs(m.ROAS-5.0) > 0.001 {
		t.Errorf("ROAS = %v, want 5.0", m.ROAS)
	}
	if math.Abs(m.AvgCPC-2.0) > 0.001 {
		t.Errorf("AvgCPC = %v, want 2.0", m.AvgCPC)
	}
}
