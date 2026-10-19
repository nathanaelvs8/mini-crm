// Package store defines what the HTTP layer needs from a database.
// Two implementations exist: Memory (for tests and quick demos) and
// pg.Store (PostgreSQL, used in production).
package store

import (
	"context"
	"errors"

	"crm/internal/model"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

// Every customer/interaction method takes ownerID. A user can only ever
// touch their own rows, so a wrong ID returns ErrNotFound (never "forbidden",
// which would leak that the row exists).
type Store interface {
	CreateUser(ctx context.Context, u *model.User) error
	UserByEmail(ctx context.Context, email string) (*model.User, error)
	UserByID(ctx context.Context, id int64) (*model.User, error)

	ListCustomers(ctx context.Context, ownerID int64, query, stage string) ([]model.Customer, error)
	CreateCustomer(ctx context.Context, c *model.Customer) error
	GetCustomer(ctx context.Context, ownerID, id int64) (*model.Customer, error)
	UpdateCustomer(ctx context.Context, c *model.Customer) error
	SetStage(ctx context.Context, ownerID, id int64, stage string) error
	DeleteCustomer(ctx context.Context, ownerID, id int64) error

	ListInteractions(ctx context.Context, ownerID, customerID int64) ([]model.Interaction, error)
	AddInteraction(ctx context.Context, ownerID int64, in *model.Interaction) error
	DeleteInteraction(ctx context.Context, ownerID, id int64) error

	// Stats returns the number of customers in each stage (missing stages = 0).
	Stats(ctx context.Context, ownerID int64) (map[string]int, error)
}
