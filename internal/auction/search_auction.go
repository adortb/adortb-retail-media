// Package auction 实现搜索页和分类页广告位拍卖。
package auction

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/adortb/adortb-retail-media/internal/campaign"
	"github.com/adortb/adortb-retail-media/internal/catalog"
	"github.com/adortb/adortb-retail-media/internal/search"
)

// ErrNoAds 表示没有满足条件的广告。
var ErrNoAds = errors.New("auction: no eligible ads")

// AdType 广告类型。
type AdType string

const (
	AdTypeSP AdType = "sp"
	AdTypeSB AdType = "sb"
)

// Ad 广告结果。
type Ad struct {
	Type        AdType                 `json:"type"`
	SKU         string                 `json:"sku,omitempty"`
	AdID        string                 `json:"ad_id"`
	Price       float64                `json:"price,omitempty"`
	Title       string                 `json:"title,omitempty"`
	ImageURL    string                 `json:"image_url,omitempty"`
	ProductURL  string                 `json:"product_url,omitempty"`
	Brand       string                 `json:"brand,omitempty"`
	Headline    string                 `json:"headline,omitempty"`
	LogoURL     string                 `json:"logo_url,omitempty"`
	LandingPage string                 `json:"landing_page,omitempty"`
	SKUs        []string               `json:"skus,omitempty"`
	CPC         float64                `json:"cpc"` // 实际扣费价格
	Markup      map[string]interface{} `json:"markup,omitempty"`
	AdGroupID   int64                  `json:"-"`
	KeywordID   int64                  `json:"-"`
	AdvertiserID int64                 `json:"-"`
}

// SearchRequest 搜索广告请求。
type SearchRequest struct {
	Query       string `json:"query"`
	UserID      string `json:"user_id"`
	Placement   string `json:"placement"` // search_top / search_middle / search_bottom
	SlotCount   int    `json:"slot_count"`
	CategoryID  string `json:"category_id,omitempty"`
}

// SearchResponse 搜索广告响应。
type SearchResponse struct {
	Ads               []Ad   `json:"ads"`
	OrganicShownAfter bool   `json:"organic_shown_after"`
	RequestID         string `json:"request_id"`
}

// SearchAuction 搜索页广告拍卖引擎。
type SearchAuction struct {
	spStore      campaign.SPStore
	sbStore      campaign.SBStore
	productStore catalog.ProductStore
	inventory    catalog.InventoryStore
	budget       *campaign.BudgetGuard
}

// NewSearchAuction 创建搜索拍卖引擎。
func NewSearchAuction(
	spStore campaign.SPStore,
	sbStore campaign.SBStore,
	productStore catalog.ProductStore,
	inventory catalog.InventoryStore,
	budget *campaign.BudgetGuard,
) *SearchAuction {
	return &SearchAuction{
		spStore:      spStore,
		sbStore:      sbStore,
		productStore: productStore,
		inventory:    inventory,
		budget:       budget,
	}
}

// spCandidate 内部使用的 SP 候选。
type spCandidate struct {
	product      *catalog.Product
	keyword      *campaign.SPKeyword
	adGroupID    int64
	campaignID   int64
	advertiserID int64
	effectiveBid float64
	qualityScore float64
	rank         float64
}

// Run 执行搜索广告拍卖，返回 top N 广告。
func (a *SearchAuction) Run(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if req.SlotCount <= 0 {
		req.SlotCount = 3
	}

	pq := search.ParseQuery(req.Query)
	queryStr := pq.QueryString()

	// 1. 获取所有活跃 SP 广告组关键词（全量；生产中应加 Cache 层）
	// 通过 advertiser_id 全量检索，然后按 query 匹配
	spAds, err := a.rankSPCandidates(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("auction: rank SP: %w", err)
	}

	// 2. 取 top N SP 广告（二价定价）
	selectedSP := a.selectTopSP(spAds, req.SlotCount)

	// 3. 获取 SB 广告（简单选取匹配品牌的第一个活跃活动）
	sbAd := a.fetchSBAd(ctx, queryStr)

	// 4. 组装响应
	ads := buildAdList(selectedSP, sbAd)
	return &SearchResponse{
		Ads:               ads,
		OrganicShownAfter: true,
		RequestID:         generateRequestID(),
	}, nil
}

func (a *SearchAuction) rankSPCandidates(ctx context.Context, query string) ([]spCandidate, error) {
	// 简化实现：从 catalog 中全量查询并在内存中匹配
	// 生产环境应通过倒排索引或 DB 层的关键词匹配优化
	products, err := a.productStore.List(ctx, catalog.ProductFilter{Status: "active", Limit: 200})
	if err != nil {
		return nil, err
	}

	// 按 SKU 建索引
	productBySKU := make(map[string]*catalog.Product, len(products))
	for _, p := range products {
		productBySKU[p.SKU] = p
	}

	// 批量检查库存
	skus := make([]string, 0, len(products))
	for _, p := range products {
		skus = append(skus, p.SKU)
	}
	inStock, err := a.inventory.BulkInStock(ctx, skus)
	if err != nil {
		return nil, err
	}

	var candidates []spCandidate
	// 遍历所有活跃广告组，寻找关键词匹配
	seen := make(map[string]bool) // 防止同一 SKU 重复进入

	for _, p := range products {
		if !inStock[p.SKU] || seen[p.SKU] {
			continue
		}
		// 用商品关键词做广泛匹配模拟
		kwList := make([]search.Keyword, 0, len(p.Keywords))
		for _, kw := range p.Keywords {
			kwList = append(kwList, search.Keyword{Text: kw, MatchType: search.MatchBroad})
		}
		_, score := search.BestMatch(kwList, query)
		if score == 0 {
			continue
		}

		// 检查预算（简化：用商品均价作为出价，实际需从 ad_group 读取）
		defaultBid := p.Price * 0.05 // 默认出价 = 售价 5%
		qs := search.QualityScore(0.02, p.Rating, p.Price, p.Price)
		rank := defaultBid * qs

		candidates = append(candidates, spCandidate{
			product:      p,
			advertiserID: p.AdvertiserID,
			effectiveBid: defaultBid,
			qualityScore: qs,
			rank:         rank,
		})
		seen[p.SKU] = true
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].rank > candidates[j].rank
	})
	return candidates, nil
}

func (a *SearchAuction) selectTopSP(candidates []spCandidate, slots int) []spCandidate {
	n := slots
	if n > len(candidates) {
		n = len(candidates)
	}
	return candidates[:n]
}

func (a *SearchAuction) fetchSBAd(ctx context.Context, _ string) *campaign.SBCampaign {
	// 简化：返回第一个活跃 SB（生产中应基于品牌匹配/出价）
	// SBStore.List 需要 advertiserID，此处跳过（Demo 逻辑）
	_ = ctx
	return nil
}

func buildAdList(spCandidates []spCandidate, sbCampaign *campaign.SBCampaign) []Ad {
	var ads []Ad

	// SB 广告放顶部（如有）
	if sbCampaign != nil {
		ads = append(ads, Ad{
			Type:        AdTypeSB,
			AdID:        fmt.Sprintf("sb-%d", sbCampaign.ID),
			Brand:       sbCampaign.Brand,
			Headline:    sbCampaign.Headline,
			LogoURL:     sbCampaign.LogoURL,
			LandingPage: sbCampaign.LandingPage,
			SKUs:        sbCampaign.SKUs,
			CPC:         0,
			AdvertiserID: sbCampaign.AdvertiserID,
		})
	}

	// 计算二价 CPC
	for i, c := range spCandidates {
		nextBid := 0.0
		if i+1 < len(spCandidates) {
			nextBid = spCandidates[i+1].effectiveBid
		}
		cpc := campaign.SecondPrice(c.effectiveBid, nextBid)

		ads = append(ads, Ad{
			Type:        AdTypeSP,
			SKU:         c.product.SKU,
			AdID:        fmt.Sprintf("sp-%s-%d", c.product.SKU, c.adGroupID),
			Price:       c.product.Price,
			Title:       c.product.Title,
			ImageURL:    c.product.ImageURL,
			ProductURL:  c.product.ProductURL,
			Brand:       c.product.Brand,
			CPC:         cpc,
			AdGroupID:   c.adGroupID,
			AdvertiserID: c.advertiserID,
			Markup: map[string]interface{}{
				"sku":       c.product.SKU,
				"title":     c.product.Title,
				"price":     c.product.Price,
				"rating":    c.product.Rating,
				"image_url": c.product.ImageURL,
				"ad_type":   "sp",
			},
		})
	}
	return ads
}

// generateRequestID 生成简单请求 ID（生产中用 UUID 库）。
func generateRequestID() string {
	return fmt.Sprintf("rmn-%d", nowUnixNano())
}
