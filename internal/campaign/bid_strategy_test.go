package campaign

import (
	"math"
	"testing"
)

func TestCPCStrategy(t *testing.T) {
	s := CPCStrategy{}
	if got := s.EffectiveBid(1.5, 0.1); got != 1.5 {
		t.Errorf("CPCStrategy.EffectiveBid = %v, want 1.5", got)
	}
	if s.Mode() != BidModeCPC {
		t.Errorf("mode = %v, want cpc", s.Mode())
	}
}

func TestCPAStrategy(t *testing.T) {
	s := CPAStrategy{TargetCPA: 10.0, MaxBid: 2.0}
	// bid = 10 * 0.05 = 0.5
	got := s.EffectiveBid(0, 0.05)
	if math.Abs(got-0.5) > 0.001 {
		t.Errorf("CPAStrategy.EffectiveBid = %v, want 0.5", got)
	}
	// bid 超过 maxBid 时钳位
	got2 := s.EffectiveBid(0, 0.5) // 10*0.5=5 > 2
	if got2 != 2.0 {
		t.Errorf("CPAStrategy.EffectiveBid capped = %v, want 2.0", got2)
	}
}

func TestCPAStrategy_ZeroCVR(t *testing.T) {
	s := CPAStrategy{TargetCPA: 10.0, MaxBid: 5.0}
	got := s.EffectiveBid(0, 0) // cvr=0，用默认 0.01
	if got <= 0 {
		t.Errorf("CPAStrategy zero cvr = %v, should be > 0", got)
	}
}

func TestSecondPrice(t *testing.T) {
	tests := []struct {
		winner float64
		next   float64
		want   float64
	}{
		{2.0, 1.0, 1.01},
		{2.0, 0.0, 0.01},
		{1.0, 1.5, 1.0}, // 第二价 > 胜者出价，钳位到胜者出价
	}
	for _, tc := range tests {
		got := SecondPrice(tc.winner, tc.next)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("SecondPrice(%v,%v) = %v, want %v", tc.winner, tc.next, got, tc.want)
		}
	}
}

func TestBudgetGuard(t *testing.T) {
	g := NewBudgetGuard(4)
	// 初始有预算
	if !g.HasBudget(1, 100.0) {
		t.Error("expected budget available initially")
	}
	// 花费后检查
	g.RecordSpend(1, 50.0)
	if !g.HasBudget(1, 100.0) {
		t.Error("expected budget available after partial spend")
	}
	g.RecordSpend(1, 60.0) // 总计 110 > 100
	if g.HasBudget(1, 100.0) {
		t.Error("expected no budget after exceeding daily limit")
	}
	// 重置后恢复
	g.ResetAll()
	if !g.HasBudget(1, 100.0) {
		t.Error("expected budget after reset")
	}
}

func TestBudgetGuard_Concurrent(t *testing.T) {
	g := NewBudgetGuard(8)
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			g.RecordSpend(1, 0.1)
			_ = g.HasBudget(1, 1000.0)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	spend := g.CurrentSpend(1)
	if math.Abs(spend-10.0) > 0.1 {
		t.Errorf("concurrent spend = %v, want ~10.0", spend)
	}
}
