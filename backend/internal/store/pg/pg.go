// Package pg is the PostgreSQL implementation of store.Store (using pgx).
package pg

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"crm/internal/model"
	"crm/internal/store"
)

//go:embed schema.sql
var schema string

type Store struct {
	pool *pgxpool.Pool
}

var _ store.Store = (*Store)(nil) // compile-time check that we implement the whole interface

// New opens a connection pool and checks that the database answers.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot reach database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Migrate creates the tables if they do not exist yet.
func (s *Store) Migrate(ctx context.Context) error {
	for _, stmt := range strings.Split(schema, ";") {
		if !hasSQL(stmt) {
			continue
		}
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// hasSQL is false for chunks that only contain whitespace and "--" comment lines.
func hasSQL(chunk string) bool {
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "--") {
			return true
		}
	}
	return false
}

// mapErr turns driver errors into the store's own errors.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	return err
}

// ---- users ----

func (s *Store) CreateUser(ctx context.Context, u *model.User) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		u.Name, u.Email, u.PasswordHash).Scan(&u.ID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return store.ErrEmailTaken
	}
	return err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

// ---- customers ----

const customerColumns = `id, owner_id, name, email, phone, company, stage, created_at, updated_at`

func scanCustomer(row pgx.Row, c *model.Customer) error {
	return row.Scan(&c.ID, &c.OwnerID, &c.Name, &c.Email, &c.Phone, &c.Company, &c.Stage, &c.CreatedAt, &c.UpdatedAt)
}

func (s *Store) ListCustomers(ctx context.Context, ownerID int64, query, stage string) ([]model.Customer, error) {
	// An empty $2 / $3 means "no filter on this".
	rows, err := s.pool.Query(ctx, `
		SELECT `+customerColumns+`
		FROM customers
		WHERE owner_id = $1
		  AND ($2::text = '' OR stage = $2::text)
		  AND ($3::text = '' OR name ILIKE '%' || $3::text || '%'
		                     OR email ILIKE '%' || $3::text || '%'
		                     OR company ILIKE '%' || $3::text || '%')
		ORDER BY id DESC`, ownerID, stage, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Customer{} // empty slice (not nil) so JSON becomes [] and not null
	for rows.Next() {
		var c model.Customer
		if err := scanCustomer(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCustomer(ctx context.Context, c *model.Customer) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO customers (owner_id, name, email, phone, company, stage)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		c.OwnerID, c.Name, c.Email, c.Phone, c.Company, c.Stage).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (s *Store) GetCustomer(ctx context.Context, ownerID, id int64) (*model.Customer, error) {
	var c model.Customer
	err := scanCustomer(s.pool.QueryRow(ctx,
		`SELECT `+customerColumns+` FROM customers WHERE id = $1 AND owner_id = $2`, id, ownerID), &c)
	if err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (s *Store) UpdateCustomer(ctx context.Context, c *model.Customer) error {
	err := s.pool.QueryRow(ctx, `
		UPDATE customers
		SET name = $3, email = $4, phone = $5, company = $6, updated_at = now()
		WHERE id = $1 AND owner_id = $2
		RETURNING stage, created_at, updated_at`,
		c.ID, c.OwnerID, c.Name, c.Email, c.Phone, c.Company).
		Scan(&c.Stage, &c.CreatedAt, &c.UpdatedAt)
	return mapErr(err)
}

func (s *Store) SetStage(ctx context.Context, ownerID, id int64, stage string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE customers SET stage = $3, updated_at = now() WHERE id = $1 AND owner_id = $2`,
		id, ownerID, stage)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteCustomer(ctx context.Context, ownerID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM customers WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ---- interactions ----

func (s *Store) ListInteractions(ctx context.Context, ownerID, customerID int64) ([]model.Interaction, error) {
	// Check the customer belongs to this user first, so a stranger gets 404 instead of an empty list.
	if _, err := s.GetCustomer(ctx, ownerID, customerID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, customer_id, kind, content, created_at
		FROM interactions
		WHERE customer_id = $1
		ORDER BY id DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Interaction{}
	for rows.Next() {
		var it model.Interaction
		if err := rows.Scan(&it.ID, &it.CustomerID, &it.Kind, &it.Content, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) AddInteraction(ctx context.Context, ownerID int64, it *model.Interaction) error {
	// INSERT ... SELECT only inserts when the customer is owned by this user.
	err := s.pool.QueryRow(ctx, `
		INSERT INTO interactions (customer_id, kind, content)
		SELECT c.id, $3::text, $4::text FROM customers c WHERE c.id = $1 AND c.owner_id = $2
		RETURNING id, created_at`,
		it.CustomerID, ownerID, it.Kind, it.Content).
		Scan(&it.ID, &it.CreatedAt)
	return mapErr(err)
}

func (s *Store) DeleteInteraction(ctx context.Context, ownerID, id int64) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM interactions i
		USING customers c
		WHERE i.id = $1 AND i.customer_id = c.id AND c.owner_id = $2`, id, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ---- stats ----

func (s *Store) Stats(ctx context.Context, ownerID int64) (map[string]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT stage, count(*) FROM customers WHERE owner_id = $1 GROUP BY stage`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var stage string
		var n int
		if err := rows.Scan(&stage, &n); err != nil {
			return nil, err
		}
		out[stage] = n
	}
	return out, rows.Err()
}
