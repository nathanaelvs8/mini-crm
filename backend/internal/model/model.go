// Package model holds the plain data types shared by every layer.
package model

import (
	"slices"
	"time"
)

// Stages is the sales pipeline, in order.
var Stages = []string{"lead", "contacted", "negotiation", "won", "lost"}

// Kinds are the types of interaction we can log for a customer.
var Kinds = []string{"call", "email", "meeting", "note"}

func ValidStage(s string) bool { return slices.Contains(Stages, s) }
func ValidKind(k string) bool  { return slices.Contains(Kinds, k) }

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

type Customer struct {
	ID        int64     `json:"id"`
	OwnerID   int64     `json:"-"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Company   string    `json:"company"`
	Stage     string    `json:"stage"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Interaction struct {
	ID         int64     `json:"id"`
	CustomerID int64     `json:"customer_id"`
	Kind       string    `json:"kind"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}
