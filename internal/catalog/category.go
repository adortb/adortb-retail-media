package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Category 类目节点。
type Category struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

// CategoryStore 类目树持久化接口。
type CategoryStore interface {
	Get(ctx context.Context, id string) (*Category, error)
	Children(ctx context.Context, parentID string) ([]*Category, error)
	Upsert(ctx context.Context, c *Category) error
	Ancestors(ctx context.Context, id string) ([]*Category, error)
}

type pgCategoryStore struct {
	db *sql.DB
}

// NewCategoryStore 创建 PostgreSQL 类目存储。
func NewCategoryStore(db *sql.DB) CategoryStore {
	return &pgCategoryStore{db: db}
}

func (s *pgCategoryStore) Get(ctx context.Context, id string) (*Category, error) {
	const q = `SELECT id, parent_id, name, level, path, created_at FROM rmn_categories WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanCategory(row)
}

func (s *pgCategoryStore) Children(ctx context.Context, parentID string) ([]*Category, error) {
	const q = `SELECT id, parent_id, name, level, path, created_at FROM rmn_categories WHERE parent_id = $1 ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q, parentID)
	if err != nil {
		return nil, fmt.Errorf("catalog: children query: %w", err)
	}
	defer rows.Close()
	return collectCategories(rows)
}

func (s *pgCategoryStore) Upsert(ctx context.Context, c *Category) error {
	if c == nil || c.ID == "" || c.Name == "" {
		return fmt.Errorf("catalog: invalid category")
	}
	const q = `INSERT INTO rmn_categories (id, parent_id, name, level, path)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET parent_id=EXCLUDED.parent_id, name=EXCLUDED.name,
		  level=EXCLUDED.level, path=EXCLUDED.path`
	_, err := s.db.ExecContext(ctx, q, c.ID, nullStr(c.ParentID), c.Name, c.Level, c.Path)
	return err
}

// Ancestors 返回从根到父节点的路径（通过 path 列解析）。
func (s *pgCategoryStore) Ancestors(ctx context.Context, id string) ([]*Category, error) {
	cat, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cat.Path == "" {
		return nil, nil
	}
	parts := strings.Split(cat.Path, "/")
	if len(parts) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(parts))
	args := make([]interface{}, len(parts))
	for i, p := range parts {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = p
	}
	q := fmt.Sprintf(`SELECT id, parent_id, name, level, path, created_at FROM rmn_categories
		WHERE id IN (%s) ORDER BY level`, strings.Join(placeholders, ","))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("catalog: ancestors query: %w", err)
	}
	defer rows.Close()
	return collectCategories(rows)
}

func scanCategory(s scanner) (*Category, error) {
	c := &Category{}
	var parentID, path sql.NullString
	err := s.Scan(&c.ID, &parentID, &c.Name, &c.Level, &path, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("catalog: category not found")
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: scan category: %w", err)
	}
	c.ParentID = parentID.String
	c.Path = path.String
	return c, nil
}

func collectCategories(rows *sql.Rows) ([]*Category, error) {
	var cats []*Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
