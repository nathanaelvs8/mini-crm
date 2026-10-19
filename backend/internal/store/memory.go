package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"crm/internal/model"
)

// Memory keeps everything in maps. Data is lost on restart, so it is only
// meant for tests and for trying the app without installing PostgreSQL.
type Memory struct {
	mu           sync.Mutex
	users        map[int64]*model.User
	customers    map[int64]*model.Customer
	interactions map[int64]*model.Interaction
	nextID       int64
}

func NewMemory() *Memory {
	return &Memory{
		users:        map[int64]*model.User{},
		customers:    map[int64]*model.Customer{},
		interactions: map[int64]*model.Interaction{},
	}
}

func (m *Memory) id() int64 { m.nextID++; return m.nextID }

func (m *Memory) CreateUser(_ context.Context, u *model.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.users {
		if existing.Email == u.Email {
			return ErrEmailTaken
		}
	}
	u.ID = m.id()
	cp := *u
	m.users[u.ID] = &cp
	return nil
}

func (m *Memory) UserByEmail(_ context.Context, email string) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) UserByID(_ context.Context, id int64) (*model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) ListCustomers(_ context.Context, ownerID int64, query, stage string) ([]model.Customer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := strings.ToLower(query)
	out := []model.Customer{}
	for _, c := range m.customers {
		if c.OwnerID != ownerID || (stage != "" && c.Stage != stage) {
			continue
		}
		if q != "" {
			hay := strings.ToLower(c.Name + " " + c.Email + " " + c.Company)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, *c)
	}
	// newest first, same order as the SQL version
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *Memory) CreateCustomer(_ context.Context, c *model.Customer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	c.ID = m.id()
	c.CreatedAt, c.UpdatedAt = now, now
	cp := *c
	m.customers[c.ID] = &cp
	return nil
}

func (m *Memory) owned(ownerID, id int64) (*model.Customer, error) {
	c, ok := m.customers[id]
	if !ok || c.OwnerID != ownerID {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *Memory) GetCustomer(_ context.Context, ownerID, id int64) (*model.Customer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.owned(ownerID, id)
	if err != nil {
		return nil, err
	}
	cp := *c
	return &cp, nil
}

func (m *Memory) UpdateCustomer(_ context.Context, in *model.Customer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.owned(in.OwnerID, in.ID)
	if err != nil {
		return err
	}
	c.Name, c.Email, c.Phone, c.Company = in.Name, in.Email, in.Phone, in.Company
	c.UpdatedAt = time.Now().UTC()
	*in = *c
	return nil
}

func (m *Memory) SetStage(_ context.Context, ownerID, id int64, stage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.owned(ownerID, id)
	if err != nil {
		return err
	}
	c.Stage = stage
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *Memory) DeleteCustomer(_ context.Context, ownerID, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.owned(ownerID, id); err != nil {
		return err
	}
	delete(m.customers, id)
	for iid, in := range m.interactions { // same as ON DELETE CASCADE
		if in.CustomerID == id {
			delete(m.interactions, iid)
		}
	}
	return nil
}

func (m *Memory) ListInteractions(_ context.Context, ownerID, customerID int64) ([]model.Interaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.owned(ownerID, customerID); err != nil {
		return nil, err
	}
	out := []model.Interaction{}
	for _, in := range m.interactions {
		if in.CustomerID == customerID {
			out = append(out, *in)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *Memory) AddInteraction(_ context.Context, ownerID int64, in *model.Interaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.owned(ownerID, in.CustomerID); err != nil {
		return err
	}
	in.ID = m.id()
	in.CreatedAt = time.Now().UTC()
	cp := *in
	m.interactions[in.ID] = &cp
	return nil
}

func (m *Memory) DeleteInteraction(_ context.Context, ownerID, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ok := m.interactions[id]
	if !ok {
		return ErrNotFound
	}
	if _, err := m.owned(ownerID, in.CustomerID); err != nil {
		return err
	}
	delete(m.interactions, id)
	return nil
}

func (m *Memory) Stats(_ context.Context, ownerID int64) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]int{}
	for _, c := range m.customers {
		if c.OwnerID == ownerID {
			out[c.Stage]++
		}
	}
	return out, nil
}
