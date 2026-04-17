package campaign

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

// SBCampaign Sponsored Brand 广告活动。
type SBCampaign struct {
	ID           int64     `json:"id"`
	AdvertiserID int64     `json:"advertiser_id"`
	Brand        string    `json:"brand"`
	Headline     string    `json:"headline"`
	LogoURL      string    `json:"logo_url,omitempty"`
	LandingPage  string    `json:"landing_page,omitempty"`
	SKUs         []string  `json:"skus,omitempty"`
	DailyBudget  float64   `json:"daily_budget"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

var ErrSBCampaignNotFound = errors.New("campaign: sb campaign not found")

// SBStore SB 活动持久化接口。
type SBStore interface {
	Get(ctx context.Context, id int64) (*SBCampaign, error)
	List(ctx context.Context, advertiserID int64) ([]*SBCampaign, error)
	Create(ctx context.Context, c *SBCampaign) (int64, error)
	Update(ctx context.Context, c *SBCampaign) error
	Delete(ctx context.Context, id int64) error
	ListActiveBySKU(ctx context.Context, sku string) ([]*SBCampaign, error)
}

type pgSBStore struct {
	db *sql.DB
}

// NewSBStore 创建 PostgreSQL SB 存储。
func NewSBStore(db *sql.DB) SBStore {
	return &pgSBStore{db: db}
}

func (s *pgSBStore) Get(ctx context.Context, id int64) (*SBCampaign, error) {
	const q = `SELECT id, advertiser_id, brand, headline, logo_url, landing_page, skus, daily_budget, status, created_at
		FROM rmn_sb_campaigns WHERE id=$1`
	return scanSB(s.db.QueryRowContext(ctx, q, id))
}

func (s *pgSBStore) List(ctx context.Context, advertiserID int64) ([]*SBCampaign, error) {
	const q = `SELECT id, advertiser_id, brand, headline, logo_url, landing_page, skus, daily_budget, status, created_at
		FROM rmn_sb_campaigns WHERE advertiser_id=$1 ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, advertiserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSB(rows)
}

func (s *pgSBStore) Create(ctx context.Context, c *SBCampaign) (int64, error) {
	const q = `INSERT INTO rmn_sb_campaigns
		(advertiser_id, brand, headline, logo_url, landing_page, skus, daily_budget, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		c.AdvertiserID, c.Brand, c.Headline, nullStr(c.LogoURL), nullStr(c.LandingPage),
		pq.Array(c.SKUs), c.DailyBudget, statusOrDefault(c.Status)).Scan(&id)
	return id, err
}

func (s *pgSBStore) Update(ctx context.Context, c *SBCampaign) error {
	const q = `UPDATE rmn_sb_campaigns SET brand=$1, headline=$2, logo_url=$3,
		landing_page=$4, skus=$5, daily_budget=$6, status=$7 WHERE id=$8`
	_, err := s.db.ExecContext(ctx, q, c.Brand, c.Headline, nullStr(c.LogoURL),
		nullStr(c.LandingPage), pq.Array(c.SKUs), c.DailyBudget, c.Status, c.ID)
	return err
}

func (s *pgSBStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rmn_sb_campaigns SET status='deleted' WHERE id=$1`, id)
	return err
}

func (s *pgSBStore) ListActiveBySKU(ctx context.Context, sku string) ([]*SBCampaign, error) {
	const q = `SELECT id, advertiser_id, brand, headline, logo_url, landing_page, skus, daily_budget, status, created_at
		FROM rmn_sb_campaigns WHERE status='active' AND $1 = ANY(skus)`
	rows, err := s.db.QueryContext(ctx, q, sku)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSB(rows)
}

func scanSB(row interface{ Scan(...interface{}) error }) (*SBCampaign, error) {
	c := &SBCampaign{}
	var logoURL, landingPage sql.NullString
	var skus pq.StringArray
	err := row.Scan(&c.ID, &c.AdvertiserID, &c.Brand, &c.Headline,
		&logoURL, &landingPage, &skus, &c.DailyBudget, &c.Status, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrSBCampaignNotFound
	}
	if err != nil {
		return nil, err
	}
	c.LogoURL = logoURL.String
	c.LandingPage = landingPage.String
	c.SKUs = []string(skus)
	return c, nil
}

func collectSB(rows *sql.Rows) ([]*SBCampaign, error) {
	var result []*SBCampaign
	for rows.Next() {
		c, err := scanSB(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
