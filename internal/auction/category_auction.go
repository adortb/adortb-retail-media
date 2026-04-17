package auction

import (
	"context"
	"fmt"

	"github.com/adortb/adortb-retail-media/internal/campaign"
	"github.com/adortb/adortb-retail-media/internal/catalog"
)

// CategoryRequest 分类页广告请求。
type CategoryRequest struct {
	CategoryID string `json:"category_id"`
	UserID     string `json:"user_id"`
	SlotCount  int    `json:"slot_count"`
}

// CategoryAuction 分类页广告拍卖。
type CategoryAuction struct {
	productStore catalog.ProductStore
	inventory    catalog.InventoryStore
	budget       *campaign.BudgetGuard
}

// NewCategoryAuction 创建分类页拍卖引擎。
func NewCategoryAuction(
	productStore catalog.ProductStore,
	inventory catalog.InventoryStore,
	budget *campaign.BudgetGuard,
) *CategoryAuction {
	return &CategoryAuction{
		productStore: productStore,
		inventory:    inventory,
		budget:       budget,
	}
}

// Run 执行分类页广告拍卖，返回分类内 top N SP 广告。
func (a *CategoryAuction) Run(ctx context.Context, req CategoryRequest) ([]Ad, error) {
	if req.SlotCount <= 0 {
		req.SlotCount = 4
	}

	products, err := a.productStore.List(ctx, catalog.ProductFilter{
		CategoryID: req.CategoryID,
		Status:     "active",
		Limit:      100,
	})
	if err != nil {
		return nil, fmt.Errorf("category auction: list products: %w", err)
	}
	if len(products) == 0 {
		return []Ad{}, nil
	}

	skus := make([]string, len(products))
	for i, p := range products {
		skus[i] = p.SKU
	}
	inStock, err := a.inventory.BulkInStock(ctx, skus)
	if err != nil {
		return nil, err
	}

	// 按评分×价格竞争力排序
	type scored struct {
		p     *catalog.Product
		score float64
	}
	var eligible []scored
	for _, p := range products {
		if !inStock[p.SKU] {
			continue
		}
		score := p.Rating * (1.0 / (p.Price + 1))
		eligible = append(eligible, scored{p: p, score: score})
	}

	// 简单降序
	for i := 0; i < len(eligible)-1; i++ {
		for j := i + 1; j < len(eligible); j++ {
			if eligible[j].score > eligible[i].score {
				eligible[i], eligible[j] = eligible[j], eligible[i]
			}
		}
	}

	n := req.SlotCount
	if n > len(eligible) {
		n = len(eligible)
	}

	ads := make([]Ad, n)
	for i, e := range eligible[:n] {
		defaultBid := e.p.Price * 0.05
		cpc := campaign.SecondPrice(defaultBid, 0)
		ads[i] = Ad{
			Type:       AdTypeSP,
			SKU:        e.p.SKU,
			AdID:       fmt.Sprintf("cat-%s-%d", e.p.SKU, i),
			Price:      e.p.Price,
			Title:      e.p.Title,
			ImageURL:   e.p.ImageURL,
			ProductURL: e.p.ProductURL,
			Brand:      e.p.Brand,
			CPC:        cpc,
			AdvertiserID: e.p.AdvertiserID,
		}
	}
	return ads, nil
}
