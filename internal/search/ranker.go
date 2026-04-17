package search

import (
	"math"
	"sort"

	"github.com/adortb/adortb-retail-media/internal/catalog"
)

// RankedItem 排序后的结果条目（广告或自然结果）。
type RankedItem struct {
	Product   *catalog.Product
	IsAd      bool
	AdScore   float64 // bid × QualityScore
	AdSlot    int     // 0=未分配，1~N=广告位编号
	QualityScore float64
}

// QualityScore 计算广告质量分数。
// QS = CTR历史 × 商品评分系数 × 价格竞争力系数。
func QualityScore(ctrHistory float64, rating float64, price float64, avgCategoryPrice float64) float64 {
	if ctrHistory <= 0 {
		ctrHistory = 0.01 // 新广告默认 CTR
	}
	// 评分系数：5分满分，归一化到 [0.5, 1.5]
	ratingFactor := 0.5 + (rating/5.0)
	// 价格竞争力：低于均价有加成
	priceFactor := 1.0
	if avgCategoryPrice > 0 && price > 0 {
		priceFactor = math.Min(2.0, avgCategoryPrice/price)
	}
	return ctrHistory * ratingFactor * priceFactor
}

// AdCandidate 广告候选。
type AdCandidate struct {
	Product      *catalog.Product
	Bid          float64
	CTRHistory   float64
	AvgCatPrice  float64
	AdGroupID    int64
	KeywordScore int // 关键词匹配得分（3=exact, 2=phrase, 1=broad）
}

// RankAds 对广告候选按 Rank = bid × QS 排序，返回 top N。
func RankAds(candidates []AdCandidate, slots int) []RankedItem {
	if slots <= 0 || len(candidates) == 0 {
		return nil
	}
	items := make([]RankedItem, 0, len(candidates))
	for _, c := range candidates {
		qs := QualityScore(c.CTRHistory, c.Product.Rating, c.Product.Price, c.AvgCatPrice)
		// 关键词匹配质量加成
		qs *= (1.0 + float64(c.KeywordScore)*0.1)
		items = append(items, RankedItem{
			Product:      c.Product,
			IsAd:         true,
			AdScore:      c.Bid * qs,
			QualityScore: qs,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].AdScore > items[j].AdScore
	})
	n := slots
	if n > len(items) {
		n = len(items)
	}
	result := items[:n]
	for i := range result {
		result[i].AdSlot = i + 1
	}
	return result
}
