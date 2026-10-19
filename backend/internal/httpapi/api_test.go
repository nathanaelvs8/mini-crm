package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"crm/internal/store"
)

// call sends one request to the API and decodes the JSON answer into a map (or slice).
type env struct {
	t   *testing.T
	srv http.Handler
}

func newEnv(t *testing.T) *env {
	return &env{t: t, srv: New(store.NewMemory(), "test-secret-123456", "")}
}

func (e *env) call(method, path, token string, body any) (int, map[string]any, []any) {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)

	raw := bytes.TrimSpace(rec.Body.Bytes())
	var obj map[string]any
	var arr []any
	if len(raw) > 0 && raw[0] == '[' {
		_ = json.Unmarshal(raw, &arr)
	} else if len(raw) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}
	return rec.Code, obj, arr
}

func (e *env) register(name, email string) string {
	e.t.Helper()
	code, obj, _ := e.call("POST", "/api/auth/register", "", map[string]string{"name": name, "email": email, "password": "password123"})
	if code != http.StatusCreated {
		e.t.Fatalf("register %s: status %d body %v", email, code, obj)
	}
	return obj["token"].(string)
}

func (e *env) newCustomer(token, name string) int {
	e.t.Helper()
	code, obj, _ := e.call("POST", "/api/customers", token, map[string]string{"name": name, "company": "Acme"})
	if code != http.StatusCreated {
		e.t.Fatalf("create customer: status %d body %v", code, obj)
	}
	return int(obj["id"].(float64))
}

func path(id int, suffix string) string { return "/api/customers/" + strconv.Itoa(id) + suffix }

func TestAuthFlow(t *testing.T) {
	e := newEnv(t)
	e.register("Ana", "ana@example.com")

	// duplicate email
	code, _, _ := e.call("POST", "/api/auth/register", "", map[string]string{"name": "Ana 2", "email": "ANA@example.com", "password": "password123"})
	if code != http.StatusConflict {
		t.Errorf("duplicate email: want 409, got %d", code)
	}
	// validation
	code, _, _ = e.call("POST", "/api/auth/register", "", map[string]string{"name": "X", "email": "nope", "password": "password123"})
	if code != http.StatusBadRequest {
		t.Errorf("bad email: want 400, got %d", code)
	}
	code, _, _ = e.call("POST", "/api/auth/register", "", map[string]string{"name": "X", "email": "x@example.com", "password": "short"})
	if code != http.StatusBadRequest {
		t.Errorf("short password: want 400, got %d", code)
	}
	// login ok / wrong password / unknown user
	code, obj, _ := e.call("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "password123"})
	if code != http.StatusOK || obj["token"] == nil {
		t.Fatalf("login: status %d body %v", code, obj)
	}
	code, _, _ = e.call("POST", "/api/auth/login", "", map[string]string{"email": "ana@example.com", "password": "wrong-password"})
	if code != http.StatusUnauthorized {
		t.Errorf("wrong password: want 401, got %d", code)
	}
	code, _, _ = e.call("POST", "/api/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "password123"})
	if code != http.StatusUnauthorized {
		t.Errorf("unknown user: want 401, got %d", code)
	}
	// /me
	token := obj["token"].(string)
	code, me, _ := e.call("GET", "/api/me", token, nil)
	if code != http.StatusOK || me["email"] != "ana@example.com" {
		t.Errorf("me: status %d body %v", code, me)
	}
	if _, leaked := me["PasswordHash"]; leaked {
		t.Error("password hash must never be returned")
	}
}

func TestProtectedRoutesNeedToken(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/api/customers", "/api/stats", "/api/me"} {
		if code, _, _ := e.call("GET", p, "", nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s without token: want 401, got %d", p, code)
		}
		if code, _, _ := e.call("GET", p, "garbage.token.here", nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s with bad token: want 401, got %d", p, code)
		}
	}
}

func TestCustomerCRUDSearchAndStage(t *testing.T) {
	e := newEnv(t)
	token := e.register("Ana", "ana@example.com")

	id := e.newCustomer(token, "Budi Santoso")
	e.newCustomer(token, "Citra Dewi")

	// validation
	if code, _, _ := e.call("POST", "/api/customers", token, map[string]string{"name": " "}); code != http.StatusBadRequest {
		t.Errorf("empty name: want 400, got %d", code)
	}
	if code, _, _ := e.call("POST", "/api/customers", token, map[string]string{"name": "Dodi", "stage": "bogus"}); code != http.StatusBadRequest {
		t.Errorf("bad stage: want 400, got %d", code)
	}

	// default stage is lead
	_, c, _ := e.call("GET", path(id, ""), token, nil)
	if c["stage"] != "lead" {
		t.Errorf("default stage: want lead, got %v", c["stage"])
	}

	// search is case-insensitive
	_, _, list := e.call("GET", "/api/customers?q=budi", token, nil)
	if len(list) != 1 {
		t.Errorf("search budi: want 1 result, got %d", len(list))
	}
	_, _, list = e.call("GET", "/api/customers", token, nil)
	if len(list) != 2 {
		t.Errorf("list: want 2, got %d", len(list))
	}

	// update
	code, upd, _ := e.call("PUT", path(id, ""), token, map[string]string{"name": "Budi S.", "email": "budi@acme.id", "phone": "0812", "company": "Acme"})
	if code != http.StatusOK || upd["name"] != "Budi S." || upd["stage"] != "lead" {
		t.Errorf("update: status %d body %v", code, upd)
	}

	// move through the pipeline and filter by stage
	code, moved, _ := e.call("PATCH", path(id, "/stage"), token, map[string]string{"stage": "negotiation"})
	if code != http.StatusOK || moved["stage"] != "negotiation" {
		t.Errorf("set stage: status %d body %v", code, moved)
	}
	if code, _, _ := e.call("PATCH", path(id, "/stage"), token, map[string]string{"stage": "bogus"}); code != http.StatusBadRequest {
		t.Errorf("bad stage patch: want 400, got %d", code)
	}
	_, _, list = e.call("GET", "/api/customers?stage=negotiation", token, nil)
	if len(list) != 1 {
		t.Errorf("filter negotiation: want 1, got %d", len(list))
	}

	// stats include every stage
	_, st, _ := e.call("GET", "/api/stats", token, nil)
	by := st["by_stage"].(map[string]any)
	if st["total"].(float64) != 2 || by["negotiation"].(float64) != 1 || by["lead"].(float64) != 1 || by["won"].(float64) != 0 {
		t.Errorf("stats wrong: %v", st)
	}

	// delete
	if code, _, _ := e.call("DELETE", path(id, ""), token, nil); code != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", code)
	}
	if code, _, _ := e.call("GET", path(id, ""), token, nil); code != http.StatusNotFound {
		t.Errorf("get after delete: want 404, got %d", code)
	}
}

func TestInteractions(t *testing.T) {
	e := newEnv(t)
	token := e.register("Ana", "ana@example.com")
	id := e.newCustomer(token, "Budi")

	code, in, _ := e.call("POST", path(id, "/interactions"), token, map[string]string{"kind": "call", "content": "Intro call, wants a demo"})
	if code != http.StatusCreated {
		t.Fatalf("add interaction: status %d body %v", code, in)
	}
	if code, _, _ := e.call("POST", path(id, "/interactions"), token, map[string]string{"kind": "fax", "content": "x"}); code != http.StatusBadRequest {
		t.Errorf("bad kind: want 400, got %d", code)
	}
	if code, _, _ := e.call("POST", path(id, "/interactions"), token, map[string]string{"kind": "note", "content": "  "}); code != http.StatusBadRequest {
		t.Errorf("empty content: want 400, got %d", code)
	}

	_, _, list := e.call("GET", path(id, "/interactions"), token, nil)
	if len(list) != 1 {
		t.Fatalf("list interactions: want 1, got %d", len(list))
	}

	iid := strconv.Itoa(int(in["id"].(float64)))
	if code, _, _ := e.call("DELETE", "/api/interactions/"+iid, token, nil); code != http.StatusNoContent {
		t.Errorf("delete interaction: want 204, got %d", code)
	}
	_, _, list = e.call("GET", path(id, "/interactions"), token, nil)
	if len(list) != 0 {
		t.Errorf("after delete: want 0, got %d", len(list))
	}
}

// The most important security test: one user must never reach another user's data.
func TestUsersAreIsolated(t *testing.T) {
	e := newEnv(t)
	ana := e.register("Ana", "ana@example.com")
	bob := e.register("Bob", "bob@example.com")

	id := e.newCustomer(ana, "Ana's customer")
	_, in, _ := e.call("POST", path(id, "/interactions"), ana, map[string]string{"kind": "note", "content": "secret"})
	iid := strconv.Itoa(int(in["id"].(float64)))

	checks := []struct {
		name, method, url string
		body              any
	}{
		{"get", "GET", path(id, ""), nil},
		{"update", "PUT", path(id, ""), map[string]string{"name": "hacked"}},
		{"stage", "PATCH", path(id, "/stage"), map[string]string{"stage": "won"}},
		{"list interactions", "GET", path(id, "/interactions"), nil},
		{"add interaction", "POST", path(id, "/interactions"), map[string]string{"kind": "note", "content": "hi"}},
		{"delete interaction", "DELETE", "/api/interactions/" + iid, nil},
		{"delete", "DELETE", path(id, ""), nil},
	}
	for _, c := range checks {
		if code, _, _ := e.call(c.method, c.url, bob, c.body); code != http.StatusNotFound {
			t.Errorf("bob %s ana's data: want 404, got %d", c.name, code)
		}
	}

	_, _, list := e.call("GET", "/api/customers", bob, nil)
	if len(list) != 0 {
		t.Errorf("bob's list should be empty, got %d", len(list))
	}
	// and Ana's data is untouched
	_, c, _ := e.call("GET", path(id, ""), ana, nil)
	if c["name"] != "Ana's customer" || c["stage"] != "lead" {
		t.Errorf("ana's customer was changed: %v", c)
	}
}
