package catalog

import (
	"context"
	"database/sql"
	"fmt"
)

// InventoryStore 库存查询与更新接口。
type InventoryStore interface {
	// InStock 检查 SKU 是否有库存（stock_level > 0 且 status = active）。
	InStock(ctx context.Context, sku string) (bool, error)
	// UpdateStock 更新库存数量（delta 可为负数表示扣减）。
	UpdateStock(ctx context.Context, sku string, delta int) error
	// BulkInStock 批量检查，返回有库存的 SKU 集合。
	BulkInStock(ctx context.Context, skus []string) (map[string]bool, error)
}

type pgInventoryStore struct {
	db *sql.DB
}

// NewInventoryStore 创建基于 PostgreSQL 的库存存储。
func NewInventoryStore(db *sql.DB) InventoryStore {
	return &pgInventoryStore{db: db}
}

func (s *pgInventoryStore) InStock(ctx context.Context, sku string) (bool, error) {
	var level int
	var status string
	err := s.db.QueryRowContext(ctx,
		`SELECT stock_level, status FROM rmn_products WHERE sku = $1`, sku).
		Scan(&level, &status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("catalog: in stock query: %w", err)
	}
	return level > 0 && status == "active", nil
}

func (s *pgInventoryStore) UpdateStock(ctx context.Context, sku string, delta int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE rmn_products SET stock_level = GREATEST(0, stock_level + $1) WHERE sku = $2`,
		delta, sku)
	if err != nil {
		return fmt.Errorf("catalog: update stock: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (s *pgInventoryStore) BulkInStock(ctx context.Context, skus []string) (map[string]bool, error) {
	if len(skus) == 0 {
		return map[string]bool{}, nil
	}
	// 构建占位符
	placeholders := make([]interface{}, len(skus))
	params := make([]string, len(skus))
	for i, sku := range skus {
		placeholders[i] = sku
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	q := fmt.Sprintf(
		`SELECT sku, stock_level, status FROM rmn_products WHERE sku IN (%s)`,
		joinComma(params))
	rows, err := s.db.QueryContext(ctx, q, placeholders...)
	if err != nil {
		return nil, fmt.Errorf("catalog: bulk in stock: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool, len(skus))
	for rows.Next() {
		var sku, status string
		var level int
		if err := rows.Scan(&sku, &level, &status); err != nil {
			return nil, err
		}
		result[sku] = level > 0 && status == "active"
	}
	return result, rows.Err()
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
