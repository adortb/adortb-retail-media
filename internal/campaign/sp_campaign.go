// Package campaign 管理 Sponsored Product 和 Sponsored Brand 广告活动。
package campaign

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// SPCampaign Sponsored Product 广告活动。
type SPCampaign struct {
	ID            int64     `json:"id"`
	AdvertiserID  int64     `json:"advertiser_id"`
	Name          string    `json:"name"`
	TargetingType string    `json:"targeting_type"` // manual / auto
	DailyBudget   float64   `json:"daily_budget"`
	StartDate     time.Time `json:"start_date"`
	EndDate       time.Time `json:"end_date"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// SPAdGroup 广告组。
type SPAdGroup struct {
	ID         int64   `json:"id"`
	CampaignID int64   `json:"campaign_id"`
	Name       string  `json:"name"`
	DefaultBid float64 `json:"default_bid"`
	Status     string  `json:"status"`
}

// SPKeyword 关键词出价。
type SPKeyword struct {
	ID         int64   `json:"id"`
	AdGroupID  int64   `json:"ad_group_id"`
	Keyword    string  `json:"keyword"`
	MatchType  string  `json:"match_type"` // exact / phrase / broad
	Bid        float64 `json:"bid"`
	Status     string  `json:"status"`
}

// SPProductAd 推广商品。
type SPProductAd struct {
	ID        int64  `json:"id"`
	AdGroupID int64  `json:"ad_group_id"`
	SKU       string `json:"sku"`
	Status    string `json:"status"`
}

var ErrCampaignNotFound = errors.New("campaign: sp campaign not found")

// SPStore SP 活动完整持久化接口。
type SPStore interface {
	GetCampaign(ctx context.Context, id int64) (*SPCampaign, error)
	ListCampaigns(ctx context.Context, advertiserID int64) ([]*SPCampaign, error)
	CreateCampaign(ctx context.Context, c *SPCampaign) (int64, error)
	UpdateCampaign(ctx context.Context, c *SPCampaign) error
	DeleteCampaign(ctx context.Context, id int64) error

	CreateAdGroup(ctx context.Context, g *SPAdGroup) (int64, error)
	ListAdGroups(ctx context.Context, campaignID int64) ([]*SPAdGroup, error)

	CreateKeyword(ctx context.Context, kw *SPKeyword) (int64, error)
	ListKeywords(ctx context.Context, adGroupID int64) ([]*SPKeyword, error)
	// ListKeywordsByAdvertiser 用于拍卖时快速检索广告主所有活跃关键词。
	ListActiveKeywordsByAdvertiser(ctx context.Context, advertiserID int64) ([]*SPKeyword, error)

	CreateProductAd(ctx context.Context, ad *SPProductAd) (int64, error)
	ListProductAds(ctx context.Context, adGroupID int64) ([]*SPProductAd, error)
	ListActiveProductAdsByAdGroup(ctx context.Context, adGroupIDs []int64) ([]*SPProductAd, error)
}

type pgSPStore struct {
	db *sql.DB
}

// NewSPStore 创建 PostgreSQL SP 存储。
func NewSPStore(db *sql.DB) SPStore {
	return &pgSPStore{db: db}
}

func (s *pgSPStore) GetCampaign(ctx context.Context, id int64) (*SPCampaign, error) {
	const q = `SELECT id, advertiser_id, name, targeting_type, daily_budget, start_date, end_date, status, created_at
		FROM rmn_sp_campaigns WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanSPCampaign(row)
}

func (s *pgSPStore) ListCampaigns(ctx context.Context, advertiserID int64) ([]*SPCampaign, error) {
	const q = `SELECT id, advertiser_id, name, targeting_type, daily_budget, start_date, end_date, status, created_at
		FROM rmn_sp_campaigns WHERE advertiser_id = $1 ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, advertiserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SPCampaign
	for rows.Next() {
		c, err := scanSPCampaign(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *pgSPStore) CreateCampaign(ctx context.Context, c *SPCampaign) (int64, error) {
	const q = `INSERT INTO rmn_sp_campaigns (advertiser_id, name, targeting_type, daily_budget, start_date, end_date, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		c.AdvertiserID, c.Name, targetingOrDefault(c.TargetingType),
		c.DailyBudget, c.StartDate, c.EndDate, statusOrDefault(c.Status)).Scan(&id)
	return id, err
}

func (s *pgSPStore) UpdateCampaign(ctx context.Context, c *SPCampaign) error {
	const q = `UPDATE rmn_sp_campaigns SET name=$1, targeting_type=$2, daily_budget=$3,
		start_date=$4, end_date=$5, status=$6 WHERE id=$7`
	_, err := s.db.ExecContext(ctx, q, c.Name, c.TargetingType, c.DailyBudget,
		c.StartDate, c.EndDate, c.Status, c.ID)
	return err
}

func (s *pgSPStore) DeleteCampaign(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rmn_sp_campaigns SET status='deleted' WHERE id=$1`, id)
	return err
}

func (s *pgSPStore) CreateAdGroup(ctx context.Context, g *SPAdGroup) (int64, error) {
	const q = `INSERT INTO rmn_sp_ad_groups (campaign_id, name, default_bid, status)
		VALUES ($1,$2,$3,$4) RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q, g.CampaignID, g.Name, g.DefaultBid, statusOrDefault(g.Status)).Scan(&id)
	return id, err
}

func (s *pgSPStore) ListAdGroups(ctx context.Context, campaignID int64) ([]*SPAdGroup, error) {
	const q = `SELECT id, campaign_id, name, default_bid, status FROM rmn_sp_ad_groups
		WHERE campaign_id=$1 AND status!='deleted'`
	rows, err := s.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SPAdGroup
	for rows.Next() {
		g := &SPAdGroup{}
		if err := rows.Scan(&g.ID, &g.CampaignID, &g.Name, &g.DefaultBid, &g.Status); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (s *pgSPStore) CreateKeyword(ctx context.Context, kw *SPKeyword) (int64, error) {
	const q = `INSERT INTO rmn_sp_keywords (ad_group_id, keyword, match_type, bid, status)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q, kw.AdGroupID, kw.Keyword, kw.MatchType, kw.Bid, statusOrDefault(kw.Status)).Scan(&id)
	return id, err
}

func (s *pgSPStore) ListKeywords(ctx context.Context, adGroupID int64) ([]*SPKeyword, error) {
	const q = `SELECT id, ad_group_id, keyword, match_type, bid, status FROM rmn_sp_keywords
		WHERE ad_group_id=$1 AND status='active'`
	return s.queryKeywords(ctx, q, adGroupID)
}

func (s *pgSPStore) ListActiveKeywordsByAdvertiser(ctx context.Context, advertiserID int64) ([]*SPKeyword, error) {
	const q = `SELECT k.id, k.ad_group_id, k.keyword, k.match_type, k.bid, k.status
		FROM rmn_sp_keywords k
		JOIN rmn_sp_ad_groups ag ON k.ad_group_id = ag.id
		JOIN rmn_sp_campaigns c ON ag.campaign_id = c.id
		WHERE c.advertiser_id = $1 AND k.status='active' AND ag.status='active' AND c.status='active'`
	return s.queryKeywords(ctx, q, advertiserID)
}

func (s *pgSPStore) queryKeywords(ctx context.Context, q string, arg interface{}) ([]*SPKeyword, error) {
	rows, err := s.db.QueryContext(ctx, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*SPKeyword
	for rows.Next() {
		kw := &SPKeyword{}
		if err := rows.Scan(&kw.ID, &kw.AdGroupID, &kw.Keyword, &kw.MatchType, &kw.Bid, &kw.Status); err != nil {
			return nil, err
		}
		result = append(result, kw)
	}
	return result, rows.Err()
}

func (s *pgSPStore) CreateProductAd(ctx context.Context, ad *SPProductAd) (int64, error) {
	const q = `INSERT INTO rmn_sp_product_ads (ad_group_id, sku, status) VALUES ($1,$2,$3) RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q, ad.AdGroupID, ad.SKU, statusOrDefault(ad.Status)).Scan(&id)
	return id, err
}

func (s *pgSPStore) ListProductAds(ctx context.Context, adGroupID int64) ([]*SPProductAd, error) {
	const q = `SELECT id, ad_group_id, sku, status FROM rmn_sp_product_ads WHERE ad_group_id=$1 AND status='active'`
	rows, err := s.db.QueryContext(ctx, q, adGroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProductAds(rows)
}

func (s *pgSPStore) ListActiveProductAdsByAdGroup(ctx context.Context, adGroupIDs []int64) ([]*SPProductAd, error) {
	if len(adGroupIDs) == 0 {
		return nil, nil
	}
	const q = `SELECT id, ad_group_id, sku, status FROM rmn_sp_product_ads
		WHERE ad_group_id = ANY($1) AND status='active'`
	rows, err := s.db.QueryContext(ctx, q, pq.Array(adGroupIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProductAds(rows)
}

func scanSPCampaign(s interface {
	Scan(...interface{}) error
}) (*SPCampaign, error) {
	c := &SPCampaign{}
	err := s.Scan(&c.ID, &c.AdvertiserID, &c.Name, &c.TargetingType,
		&c.DailyBudget, &c.StartDate, &c.EndDate, &c.Status, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrCampaignNotFound
	}
	return c, err
}

func scanProductAds(rows *sql.Rows) ([]*SPProductAd, error) {
	var result []*SPProductAd
	for rows.Next() {
		ad := &SPProductAd{}
		if err := rows.Scan(&ad.ID, &ad.AdGroupID, &ad.SKU, &ad.Status); err != nil {
			return nil, err
		}
		result = append(result, ad)
	}
	return result, rows.Err()
}

func targetingOrDefault(t string) string {
	if t == "" {
		return "manual"
	}
	return t
}

func statusOrDefault(s string) string {
	if s == "" {
		return "active"
	}
	return s
}

// SpendTracker 追踪广告主当日花费（内存缓存，按需持久化）。
type SpendTracker struct {
	spends map[int64]float64 // advertiserID -> today spend
}

// NewSpendTracker 创建花费追踪器。
func NewSpendTracker() *SpendTracker {
	return &SpendTracker{spends: make(map[int64]float64)}
}

// HasBudget 检查广告主今日花费是否低于活动日预算。
func (t *SpendTracker) HasBudget(advertiserID int64, dailyBudget float64) bool {
	return t.spends[advertiserID] < dailyBudget
}

// RecordSpend 记录花费（CPC 出价）。
func (t *SpendTracker) RecordSpend(advertiserID int64, amount float64) {
	t.spends[advertiserID] += amount
}

// Reset 每天 0 点重置。
func (t *SpendTracker) Reset() {
	for k := range t.spends {
		t.spends[k] = 0
	}
}

// BudgetGuard 线程安全的预算检查器（多分片）。
type BudgetGuard struct {
	shards []*spendShard
	n      int
}

type spendShard struct {
	mu     chan struct{} // 用 buffered channel 模拟轻量 mutex
	spends map[int64]float64
}

// NewBudgetGuard 创建 n 分片的预算守卫。
func NewBudgetGuard(shards int) *BudgetGuard {
	if shards <= 0 {
		shards = 16
	}
	g := &BudgetGuard{shards: make([]*spendShard, shards), n: shards}
	for i := range g.shards {
		s := &spendShard{
			mu:     make(chan struct{}, 1),
			spends: make(map[int64]float64),
		}
		s.mu <- struct{}{}
		g.shards[i] = s
	}
	return g
}

func (g *BudgetGuard) shard(advertiserID int64) *spendShard {
	return g.shards[advertiserID%int64(g.n)]
}

// HasBudget 并发安全：检查预算。
func (g *BudgetGuard) HasBudget(advertiserID int64, dailyBudget float64) bool {
	s := g.shard(advertiserID)
	<-s.mu
	ok := s.spends[advertiserID] < dailyBudget
	s.mu <- struct{}{}
	return ok
}

// RecordSpend 并发安全：记录花费。
func (g *BudgetGuard) RecordSpend(advertiserID int64, amount float64) {
	s := g.shard(advertiserID)
	<-s.mu
	s.spends[advertiserID] += amount
	s.mu <- struct{}{}
}

// ResetAll 重置所有分片花费。
func (g *BudgetGuard) ResetAll() {
	for _, s := range g.shards {
		<-s.mu
		for k := range s.spends {
			s.spends[k] = 0
		}
		s.mu <- struct{}{}
	}
}

// CurrentSpend 返回当前花费（用于监控）。
func (g *BudgetGuard) CurrentSpend(advertiserID int64) float64 {
	s := g.shard(advertiserID)
	<-s.mu
	v := s.spends[advertiserID]
	s.mu <- struct{}{}
	return v
}

// ErrBudgetExhausted 预算耗尽错误。
var ErrBudgetExhausted = fmt.Errorf("campaign: daily budget exhausted")
