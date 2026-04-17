// Package reporting 提供零售媒体独有的 ACOS/ROAS/TACoS 指标计算与查询。
package reporting

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PerformanceRow 成效数据行。
type PerformanceRow struct {
	Date       time.Time `json:"date"`
	CampaignID int64     `json:"campaign_id"`
	AdGroupID  int64     `json:"ad_group_id"`
	KeywordID  int64     `json:"keyword_id,omitempty"`
	SKU        string    `json:"sku,omitempty"`
	Impressions int64    `json:"impressions"`
	Clicks      int64    `json:"clicks"`
	Spend       float64  `json:"spend"`
	Purchases   int      `json:"purchases"`
	Sales       float64  `json:"sales"`
}

// Metrics 汇总指标。
type Metrics struct {
	Impressions  int64   `json:"impressions"`
	Clicks       int64   `json:"clicks"`
	Spend        float64 `json:"spend"`
	Purchases    int64   `json:"purchases"`
	Sales        float64 `json:"sales"`
	CTR          float64 `json:"ctr"`            // clicks / impressions
	CVR          float64 `json:"cvr"`            // purchases / clicks
	ACOS         float64 `json:"acos"`           // spend / sales
	ROAS         float64 `json:"roas"`           // sales / spend
	TACoS        float64 `json:"tacos"`          // total_spend / total_sales
	AvgCPC       float64 `json:"avg_cpc"`        // spend / clicks
}

// QueryParams 查询参数。
type QueryParams struct {
	CampaignID int64
	From       time.Time
	To         time.Time
}

// Reporter 成效报告接口。
type Reporter interface {
	QueryMetrics(ctx context.Context, params QueryParams) (*Metrics, error)
	RecordImpression(ctx context.Context, row PerformanceRow) error
	RecordClick(ctx context.Context, row PerformanceRow) error
	RecordPurchase(ctx context.Context, row PerformanceRow) error
}

type pgReporter struct {
	db *sql.DB
}

// NewReporter 创建 PostgreSQL 报告实例。
func NewReporter(db *sql.DB) Reporter {
	return &pgReporter{db: db}
}

func (r *pgReporter) QueryMetrics(ctx context.Context, params QueryParams) (*Metrics, error) {
	const q = `SELECT
		COALESCE(SUM(impressions),0),
		COALESCE(SUM(clicks),0),
		COALESCE(SUM(spend),0),
		COALESCE(SUM(purchases),0),
		COALESCE(SUM(sales),0)
		FROM rmn_performance
		WHERE campaign_id=$1 AND date >= $2 AND date <= $3`
	var m Metrics
	err := r.db.QueryRowContext(ctx, q, params.CampaignID, params.From, params.To).
		Scan(&m.Impressions, &m.Clicks, &m.Spend, &m.Purchases, &m.Sales)
	if err != nil {
		return nil, fmt.Errorf("reporting: query metrics: %w", err)
	}
	m.CTR = safeDiv(float64(m.Clicks), float64(m.Impressions))
	m.CVR = safeDiv(float64(m.Purchases), float64(m.Clicks))
	m.ACOS = ACOS(m.Spend, m.Sales)
	m.ROAS = ROAS(m.Sales, m.Spend)
	m.TACoS = TACoS(m.Spend, m.Sales) // TACoS 需要 total sales，此处用广告 sales 近似
	m.AvgCPC = safeDiv(m.Spend, float64(m.Clicks))
	return &m, nil
}

func (r *pgReporter) RecordImpression(ctx context.Context, row PerformanceRow) error {
	return r.upsertRow(ctx, row, "impressions", 1)
}

func (r *pgReporter) RecordClick(ctx context.Context, row PerformanceRow) error {
	return r.upsertRow(ctx, row, "clicks", 1)
}

func (r *pgReporter) RecordPurchase(ctx context.Context, row PerformanceRow) error {
	return r.upsertPurchase(ctx, row)
}

func (r *pgReporter) upsertRow(ctx context.Context, row PerformanceRow, field string, delta int64) error {
	q := fmt.Sprintf(`INSERT INTO rmn_performance
		(date, campaign_id, ad_group_id, keyword_id, sku, %s)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (date, campaign_id, ad_group_id, keyword_id, sku)
		DO UPDATE SET %s = rmn_performance.%s + EXCLUDED.%s`, field, field, field, field)
	_, err := r.db.ExecContext(ctx, q,
		row.Date, row.CampaignID, row.AdGroupID, row.KeywordID, nullSKU(row.SKU), delta)
	return err
}

func (r *pgReporter) upsertPurchase(ctx context.Context, row PerformanceRow) error {
	const q = `INSERT INTO rmn_performance
		(date, campaign_id, ad_group_id, keyword_id, sku, purchases, sales, spend)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (date, campaign_id, ad_group_id, keyword_id, sku)
		DO UPDATE SET
		  purchases = rmn_performance.purchases + EXCLUDED.purchases,
		  sales = rmn_performance.sales + EXCLUDED.sales,
		  spend = rmn_performance.spend + EXCLUDED.spend`
	_, err := r.db.ExecContext(ctx, q,
		row.Date, row.CampaignID, row.AdGroupID, row.KeywordID, nullSKU(row.SKU),
		row.Purchases, row.Sales, row.Spend)
	return err
}

// ACOS 广告销售成本比 = Spend / Sales。
func ACOS(spend, sales float64) float64 {
	return safeDiv(spend, sales)
}

// ROAS 广告花费回报 = Sales / Spend。
func ROAS(sales, spend float64) float64 {
	return safeDiv(sales, spend)
}

// TACoS 总广告销售成本 = TotalAdSpend / TotalSales。
func TACoS(totalAdSpend, totalSales float64) float64 {
	return safeDiv(totalAdSpend, totalSales)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func nullSKU(sku string) interface{} {
	if sku == "" {
		return ""
	}
	return sku
}
