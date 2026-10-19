package httpapi

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"crm/internal/auth"
	"crm/internal/model"
	"crm/internal/store"
)

// ---- auth ----

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in registerRequest
	if !readJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		writeError(w, http.StatusBadRequest, "email is not valid")
		return
	}
	if len(in.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		serverError(w, r.Context(), err)
		return
	}
	u := &model.User{Name: in.Name, Email: in.Email, PasswordHash: hash}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email is already registered")
			return
		}
		serverError(w, r.Context(), err)
		return
	}
	s.respondWithToken(w, r, http.StatusCreated, u)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if !readJSON(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	u, err := s.store.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		serverError(w, r.Context(), err)
		return
	}
	// Same message for "no such user" and "wrong password" so attackers can't probe for emails.
	if err != nil || !auth.CheckPassword(in.Password, u.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}
	s.respondWithToken(w, r, http.StatusOK, u)
}

func (s *Server) respondWithToken(w http.ResponseWriter, r *http.Request, status int, u *model.User) {
	token, err := auth.NewToken(s.secret, u.ID, s.tokenTTL)
	if err != nil {
		serverError(w, r.Context(), err)
		return
	}
	writeJSON(w, status, authResponse{Token: token, User: u})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, userID int64) {
	u, err := s.store.UserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "user no longer exists")
			return
		}
		serverError(w, r.Context(), err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// ---- customers ----

type customerRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Stage   string `json:"stage"` // only used when creating
}

// clean trims the fields and checks the rules shared by create and update.
func (c *customerRequest) clean() string {
	c.Name = strings.TrimSpace(c.Name)
	c.Email = strings.TrimSpace(c.Email)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Company = strings.TrimSpace(c.Company)
	if c.Name == "" {
		return "name is required"
	}
	if len(c.Name) > 200 || len(c.Email) > 200 || len(c.Phone) > 50 || len(c.Company) > 200 {
		return "a field is too long"
	}
	if c.Email != "" {
		if _, err := mail.ParseAddress(c.Email); err != nil {
			return "email is not valid"
		}
	}
	return ""
}

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request, userID int64) {
	stage := r.URL.Query().Get("stage")
	if stage != "" && !model.ValidStage(stage) {
		writeError(w, http.StatusBadRequest, "unknown stage")
		return
	}
	list, err := s.store.ListCustomers(r.Context(), userID, strings.TrimSpace(r.URL.Query().Get("q")), stage)
	if err != nil {
		serverError(w, r.Context(), err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request, userID int64) {
	var in customerRequest
	if !readJSON(w, r, &in) {
		return
	}
	if msg := in.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if in.Stage == "" {
		in.Stage = "lead"
	}
	if !model.ValidStage(in.Stage) {
		writeError(w, http.StatusBadRequest, "unknown stage")
		return
	}
	c := &model.Customer{OwnerID: userID, Name: in.Name, Email: in.Email, Phone: in.Phone, Company: in.Company, Stage: in.Stage}
	if err := s.store.CreateCustomer(r.Context(), c); err != nil {
		serverError(w, r.Context(), err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := s.store.GetCustomer(r.Context(), userID, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) updateCustomer(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in customerRequest
	if !readJSON(w, r, &in) {
		return
	}
	if msg := in.clean(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	c := &model.Customer{ID: id, OwnerID: userID, Name: in.Name, Email: in.Email, Phone: in.Phone, Company: in.Company}
	if err := s.store.UpdateCustomer(r.Context(), c); err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteCustomer(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteCustomer(r.Context(), userID, id); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setStage(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Stage string `json:"stage"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !model.ValidStage(in.Stage) {
		writeError(w, http.StatusBadRequest, "unknown stage")
		return
	}
	if err := s.store.SetStage(r.Context(), userID, id, in.Stage); err != nil {
		s.storeError(w, r, err)
		return
	}
	c, err := s.store.GetCustomer(r.Context(), userID, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type statsResponse struct {
	Total   int            `json:"total"`
	ByStage map[string]int `json:"by_stage"`
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request, userID int64) {
	counts, err := s.store.Stats(r.Context(), userID)
	if err != nil {
		serverError(w, r.Context(), err)
		return
	}
	resp := statsResponse{ByStage: map[string]int{}}
	for _, st := range model.Stages { // always return every stage, even with 0
		resp.ByStage[st] = counts[st]
		resp.Total += counts[st]
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- interactions ----

func (s *Server) listInteractions(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	list, err := s.store.ListInteractions(r.Context(), userID, id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) addInteraction(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Kind    string `json:"kind"`
		Content string `json:"content"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if !model.ValidKind(in.Kind) {
		writeError(w, http.StatusBadRequest, "kind must be call, email, meeting or note")
		return
	}
	if in.Content == "" || len(in.Content) > 4000 {
		writeError(w, http.StatusBadRequest, "content is required (max 4000 characters)")
		return
	}
	it := &model.Interaction{CustomerID: id, Kind: in.Kind, Content: in.Content}
	if err := s.store.AddInteraction(r.Context(), userID, it); err != nil {
		s.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) deleteInteraction(w http.ResponseWriter, r *http.Request, userID int64) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteInteraction(r.Context(), userID, id); err != nil {
		s.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// storeError maps "not found" to 404 and everything else to 500.
func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	serverError(w, r.Context(), err)
}
