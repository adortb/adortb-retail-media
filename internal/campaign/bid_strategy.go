package campaign

import "math"

// BidMode 出价模式。
type BidMode string

const (
	BidModeCPC BidMode = "cpc" // 手动 CPC
	BidModeCPA BidMode = "cpa" // 自动 CPA 目标
)

// BidStrategy 出价策略接口。
type BidStrategy interface {
	// EffectiveBid 根据上下文（历史 CVR、目标 CPA）计算有效出价。
	EffectiveBid(base float64, cvr float64) float64
	Mode() BidMode
}

// CPCStrategy 手动 CPC：直接返回基础出价。
type CPCStrategy struct{}

func (CPCStrategy) EffectiveBid(base float64, _ float64) float64 { return base }
func (CPCStrategy) Mode() BidMode                                 { return BidModeCPC }

// CPAStrategy 自动 CPA：bid = targetCPA × estimatedCVR，上限 maxBid。
type CPAStrategy struct {
	TargetCPA float64
	MaxBid    float64
}

func (s CPAStrategy) EffectiveBid(_ float64, cvr float64) float64 {
	if cvr <= 0 {
		cvr = 0.01
	}
	bid := s.TargetCPA * cvr
	if s.MaxBid > 0 {
		bid = math.Min(bid, s.MaxBid)
	}
	return bid
}

func (CPAStrategy) Mode() BidMode { return BidModeCPA }

// NewBidStrategy 根据模式创建策略。
func NewBidStrategy(mode BidMode, targetCPA, maxBid float64) BidStrategy {
	switch mode {
	case BidModeCPA:
		return CPAStrategy{TargetCPA: targetCPA, MaxBid: maxBid}
	default:
		return CPCStrategy{}
	}
}

// SecondPrice 计算二价结算价格（Vickrey 拍卖）。
// winner 的出价，nextBidOrFloor 为第二高出价（或底价）。
func SecondPrice(winnerBid, nextBidOrFloor float64) float64 {
	price := nextBidOrFloor + 0.01
	if price > winnerBid {
		return winnerBid
	}
	return price
}
