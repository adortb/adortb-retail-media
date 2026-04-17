// Package client 提供 Retail Media Network HTTP 客户端，供其他服务集成调用。
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client RMN HTTP 客户端。
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option 客户端配置选项。
type Option func(*Client)

// WithTimeout 设置请求超时。
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// New 创建 RMN 客户端。
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SearchAdsRequest 搜索广告请求。
type SearchAdsRequest struct {
	Query     string `json:"query"`
	UserID    string `json:"user_id"`
	Placement string `json:"placement"`
	SlotCount int    `json:"slot_count"`
}

// Ad 广告返回结构。
type Ad struct {
	Type        string                 `json:"type"`
	SKU         string                 `json:"sku,omitempty"`
	AdID        string                 `json:"ad_id"`
	Price       float64                `json:"price,omitempty"`
	Title       string                 `json:"title,omitempty"`
	ImageURL    string                 `json:"image_url,omitempty"`
	Brand       string                 `json:"brand,omitempty"`
	Headline    string                 `json:"headline,omitempty"`
	SKUs        []string               `json:"skus,omitempty"`
	CPC         float64                `json:"cpc"`
	Markup      map[string]interface{} `json:"markup,omitempty"`
}

// SearchAdsResponse 搜索广告响应。
type SearchAdsResponse struct {
	Ads               []Ad   `json:"ads"`
	OrganicShownAfter bool   `json:"organic_shown_after"`
	RequestID         string `json:"request_id"`
}

// SearchAds 请求搜索页广告。
func (c *Client) SearchAds(ctx context.Context, req SearchAdsRequest) (*SearchAdsResponse, error) {
	var resp SearchAdsResponse
	if err := c.post(ctx, "/v1/ads/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CategoryAdsRequest 分类页广告请求。
type CategoryAdsRequest struct {
	CategoryID string `json:"category_id"`
	UserID     string `json:"user_id"`
	SlotCount  int    `json:"slot_count"`
}

// CategoryAdsResponse 分类页广告响应。
type CategoryAdsResponse struct {
	Ads []Ad `json:"ads"`
}

// CategoryAds 请求分类页广告。
func (c *Client) CategoryAds(ctx context.Context, req CategoryAdsRequest) (*CategoryAdsResponse, error) {
	var resp CategoryAdsResponse
	if err := c.post(ctx, "/v1/ads/category", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ClickEvent 点击事件。
type ClickEvent struct {
	CampaignID int64   `json:"campaign_id"`
	AdGroupID  int64   `json:"ad_group_id"`
	KeywordID  int64   `json:"keyword_id,omitempty"`
	SKU        string  `json:"sku,omitempty"`
	Spend      float64 `json:"spend"`
}

// RecordClick 上报点击事件。
func (c *Client) RecordClick(ctx context.Context, e ClickEvent) error {
	return c.post(ctx, "/v1/events/click", e, nil)
}

// PurchaseEvent 转化事件。
type PurchaseEvent struct {
	CampaignID int64   `json:"campaign_id"`
	AdGroupID  int64   `json:"ad_group_id"`
	SKU        string  `json:"sku,omitempty"`
	Sales      float64 `json:"sales"`
	Spend      float64 `json:"spend"`
	Purchases  int     `json:"purchases"`
}

// RecordPurchase 上报购买转化。
func (c *Client) RecordPurchase(ctx context.Context, e PurchaseEvent) error {
	return c.post(ctx, "/v1/events/purchase", e, nil)
}

func (c *Client) post(ctx context.Context, path string, body, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("rmn client: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("rmn client: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("rmn client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rmn client: status %d: %s", resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("rmn client: decode response: %w", err)
		}
	}
	return nil
}
