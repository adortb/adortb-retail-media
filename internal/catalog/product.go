// Package catalog 管理零售媒体商品目录，包含商品 CRUD、批量导入及查询。
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Product 商品信息。
type Product struct {
	SKU          string            `json:"sku"`
	AdvertiserID int64             `json:"advertiser_id"`
	Title        string            `json:"title"`
	CategoryID   string            `json:"category_id,omitempty"`
	Brand        string            `json:"brand,omitempty"`
	Price        float64           `json:"price"`
	StockLevel   int               `json:"stock_level"`
	ImageURL     string            `json:"image_url,omitempty"`
	ProductURL   string            `json:"product_url,omitempty"`
	Rating       float64           `json:"rating"`
	ReviewCount  int               `json:"review_count"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Status       string            `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ProductFilter 查询过滤参数。
type ProductFilter struct {
	AdvertiserID int64
	CategoryID   string
	Status       string
	Keyword      string
	Limit        int
	Offset       int
}

var (
	ErrProductNotFound = errors.New("catalog: product not found")
	ErrInvalidProduct  = errors.New("catalog: invalid product")
)

// ProductStore 商品持久化接口。
type ProductStore interface {
	Get(ctx context.Context, sku string) (*Product, error)
	List(ctx context.Context, f ProductFilter) ([]*Product, error)
	Upsert(ctx context.Context, p *Product) error
	Delete(ctx context.Context, sku string) error
	BulkUpsert(ctx context.Context, products []*Product) (int, error)
}

// pgProductStore 基于 PostgreSQL 的实现。
type pgProductStore struct {
	db *sql.DB
}

// NewProductStore 创建 PostgreSQL 商品存储。
func NewProductStore(db *sql.DB) ProductStore {
	return &pgProductStore{db: db}
}

func (s *pgProductStore) Get(ctx context.Context, sku string) (*Product, error) {
	const q = `SELECT sku, advertiser_id, title, category_id, brand, price, stock_level,
		image_url, product_url, rating, review_count, attributes, keywords, status, created_at
		FROM rmn_products WHERE sku = $1`
	row := s.db.QueryRowContext(ctx, q, sku)
	return scanProduct(row)
}

func (s *pgProductStore) List(ctx context.Context, f ProductFilter) ([]*Product, error) {
	var conds []string
	var args []interface{}
	idx := 1

	if f.AdvertiserID > 0 {
		conds = append(conds, fmt.Sprintf("advertiser_id = $%d", idx))
		args = append(args, f.AdvertiserID)
		idx++
	}
	if f.CategoryID != "" {
		conds = append(conds, fmt.Sprintf("category_id = $%d", idx))
		args = append(args, f.CategoryID)
		idx++
	}
	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, f.Status)
		idx++
	}
	if f.Keyword != "" {
		conds = append(conds, fmt.Sprintf("$%d = ANY(keywords)", idx))
		args = append(args, f.Keyword)
		idx++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := fmt.Sprintf(`SELECT sku, advertiser_id, title, category_id, brand, price, stock_level,
		image_url, product_url, rating, review_count, attributes, keywords, status, created_at
		FROM rmn_products %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: list products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *pgProductStore) Upsert(ctx context.Context, p *Product) error {
	if err := validateProduct(p); err != nil {
		return err
	}
	attrs, err := json.Marshal(p.Attributes)
	if err != nil {
		return fmt.Errorf("catalog: marshal attributes: %w", err)
	}
	const q = `INSERT INTO rmn_products
		(sku, advertiser_id, title, category_id, brand, price, stock_level, image_url, product_url,
		 rating, review_count, attributes, keywords, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (sku) DO UPDATE SET
		  advertiser_id=EXCLUDED.advertiser_id, title=EXCLUDED.title,
		  category_id=EXCLUDED.category_id, brand=EXCLUDED.brand, price=EXCLUDED.price,
		  stock_level=EXCLUDED.stock_level, image_url=EXCLUDED.image_url,
		  product_url=EXCLUDED.product_url, rating=EXCLUDED.rating,
		  review_count=EXCLUDED.review_count, attributes=EXCLUDED.attributes,
		  keywords=EXCLUDED.keywords, status=EXCLUDED.status`
	_, err = s.db.ExecContext(ctx, q,
		p.SKU, p.AdvertiserID, p.Title, p.CategoryID, p.Brand, p.Price, p.StockLevel,
		p.ImageURL, p.ProductURL, p.Rating, p.ReviewCount, attrs,
		pq.Array(p.Keywords), statusOrDefault(p.Status))
	return err
}

func (s *pgProductStore) Delete(ctx context.Context, sku string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rmn_products WHERE sku = $1`, sku)
	return err
}

func (s *pgProductStore) BulkUpsert(ctx context.Context, products []*Product) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	count := 0
	for _, p := range products {
		if err := validateProduct(p); err != nil {
			continue
		}
		attrs, _ := json.Marshal(p.Attributes)
		const q = `INSERT INTO rmn_products
			(sku, advertiser_id, title, category_id, brand, price, stock_level, image_url, product_url,
			 rating, review_count, attributes, keywords, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT (sku) DO UPDATE SET
			  title=EXCLUDED.title, price=EXCLUDED.price, stock_level=EXCLUDED.stock_level,
			  attributes=EXCLUDED.attributes, keywords=EXCLUDED.keywords, status=EXCLUDED.status`
		if _, err := tx.ExecContext(ctx, q,
			p.SKU, p.AdvertiserID, p.Title, p.CategoryID, p.Brand, p.Price, p.StockLevel,
			p.ImageURL, p.ProductURL, p.Rating, p.ReviewCount, attrs,
			pq.Array(p.Keywords), statusOrDefault(p.Status)); err != nil {
			return count, fmt.Errorf("catalog: bulk upsert sku %s: %w", p.SKU, err)
		}
		count++
	}
	return count, tx.Commit()
}

// scanner 抽象 sql.Row / sql.Rows 的 Scan 方法。
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanProduct(s scanner) (*Product, error) {
	p := &Product{}
	var attrs []byte
	var keywords pq.StringArray
	var categoryID, brand, imageURL, productURL sql.NullString
	err := s.Scan(&p.SKU, &p.AdvertiserID, &p.Title, &categoryID, &brand,
		&p.Price, &p.StockLevel, &imageURL, &productURL,
		&p.Rating, &p.ReviewCount, &attrs, &keywords, &p.Status, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: scan product: %w", err)
	}
	p.CategoryID = categoryID.String
	p.Brand = brand.String
	p.ImageURL = imageURL.String
	p.ProductURL = productURL.String
	p.Keywords = []string(keywords)
	if len(attrs) > 0 {
		_ = json.Unmarshal(attrs, &p.Attributes)
	}
	return p, nil
}

func validateProduct(p *Product) error {
	if p == nil || p.SKU == "" || p.Title == "" || p.AdvertiserID == 0 {
		return ErrInvalidProduct
	}
	return nil
}

func statusOrDefault(s string) string {
	if s == "" {
		return "active"
	}
	return s
}
