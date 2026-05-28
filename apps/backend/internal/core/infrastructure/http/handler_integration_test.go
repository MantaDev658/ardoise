package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"ardoise/apps/backend/internal/core/application"
	hmacauth "ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	"ardoise/apps/backend/internal/core/infrastructure/postgres"
	sharedjwt "ardoise/libs/shared/jwt"
)

const integrationTestSecret = "integration-test-secret"

func setupIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set. Skipping integration test.")
	}

	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}

	for _, stmt := range []string{
		"DELETE FROM splits",
		"DELETE FROM audit_logs",
		"DELETE FROM expenses",
		"DELETE FROM group_members",
		"DELETE FROM groups",
		"DELETE FROM users",
	} {
		if _, err := rawDB.Exec(stmt); err != nil {
			t.Fatalf("cleanup %q: %v", stmt, err)
		}
	}

	secret := []byte(integrationTestSecret)
	db := postgres.NewDB(rawDB)
	userRepo := postgres.NewUserRepository(rawDB)
	groupRepo := postgres.NewGroupRepository(rawDB)
	expenseRepo := postgres.NewExpenseRepository(rawDB)
	auditRepo := postgres.NewAuditRepository(rawDB)

	userSvc := application.NewUserService(userRepo, secret)
	groupSvc := application.NewGroupService(groupRepo, expenseRepo, auditRepo, db)
	expenseSvc := application.NewExpenseService(expenseRepo, groupRepo, auditRepo, db)

	h := NewAPIHandler(expenseSvc, userSvc, groupSvc)
	auth := hmacauth.New(secret)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /users/me", h.GetCurrentUser)
	protected.HandleFunc("GET /users", h.ListUsers)
	protected.HandleFunc("POST /groups", h.CreateGroup)
	protected.HandleFunc("GET /groups", h.ListGroups)
	protected.HandleFunc("POST /groups/{id}/members", h.AddGroupMember)
	protected.HandleFunc("POST /expenses", h.CreateExpense)
	protected.HandleFunc("GET /balances", h.GetBalances)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", h.RegisterUser)
	mux.HandleFunc("POST /auth/login", h.LoginUser)
	mux.Handle("/", AuthMiddleware(auth)(UserProvisioningMiddleware(userSvc)(protected)))

	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		rawDB.Close()
	})
	return srv
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// TestIntegration_RegisterLoginAndMe covers the canonical HMAC auth flow end-to-end.
func TestIntegration_RegisterLoginAndMe(t *testing.T) {
	srv := setupIntegrationServer(t)

	// Register
	resp := doJSON(t, srv.Client(), "POST", srv.URL+"/auth/register", map[string]string{
		"id": "alice", "display_name": "Alice", "password": "pass1234",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login
	resp = doJSON(t, srv.Client(), "POST", srv.URL+"/auth/login", map[string]string{
		"id": "alice", "password": "pass1234",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	var loginBody struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	resp.Body.Close()

	// GET /users/me
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/users/me", nil, loginBody.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /users/me: expected 200, got %d", resp.StatusCode)
	}
	var user struct {
		ID          string `json:"ID"`
		DisplayName string `json:"DisplayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	resp.Body.Close()

	if user.ID != "alice" {
		t.Errorf("expected ID alice, got %q", user.ID)
	}
	if user.DisplayName != "Alice" {
		t.Errorf("expected DisplayName Alice, got %q", user.DisplayName)
	}
}

// TestIntegration_ProvisioningOnFirstRequest verifies that UserProvisioningMiddleware
// auto-creates a user record when a new JWT sub is seen, and is idempotent on repeat.
func TestIntegration_ProvisioningOnFirstRequest(t *testing.T) {
	srv := setupIntegrationServer(t)

	tok, err := sharedjwt.Sign("newuser", []byte(integrationTestSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// First request — user doesn't exist yet; provisioning should create them.
	resp := doJSON(t, srv.Client(), "GET", srv.URL+"/users/me", nil, tok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first GET /users/me: expected 200, got %d", resp.StatusCode)
	}
	var user struct {
		ID string `json:"ID"`
	}
	json.NewDecoder(resp.Body).Decode(&user) //nolint:errcheck
	resp.Body.Close()
	if user.ID != "newuser" {
		t.Errorf("expected ID newuser, got %q", user.ID)
	}

	// Second request — provisioning must be idempotent (no duplicate-key error).
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/users/me", nil, tok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second GET /users/me: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestIntegration_GroupAndExpenseFlow exercises the full create-group → add-member →
// post-expense → get-balances path against a real database.
func TestIntegration_GroupAndExpenseFlow(t *testing.T) {
	srv := setupIntegrationServer(t)

	register := func(id, displayName string) string {
		resp := doJSON(t, srv.Client(), "POST", srv.URL+"/auth/register", map[string]string{
			"id": id, "display_name": displayName, "password": "pass1234",
		}, "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("register %s: expected 201, got %d", id, resp.StatusCode)
		}
		resp = doJSON(t, srv.Client(), "POST", srv.URL+"/auth/login", map[string]string{
			"id": id, "password": "pass1234",
		}, "")
		defer resp.Body.Close()
		var lb struct {
			Token string `json:"token"`
		}
		json.NewDecoder(resp.Body).Decode(&lb) //nolint:errcheck
		return lb.Token
	}

	aliceTok := register("alice2", "Alice")
	register("bob2", "Bob")

	// Create group as Alice
	resp := doJSON(t, srv.Client(), "POST", srv.URL+"/groups", map[string]string{"name": "Trip"}, aliceTok)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: expected 201, got %d", resp.StatusCode)
	}
	var groupResp struct {
		GroupID string `json:"group_id"`
	}
	json.NewDecoder(resp.Body).Decode(&groupResp) //nolint:errcheck
	resp.Body.Close()
	if groupResp.GroupID == "" {
		t.Fatal("expected group_id in response")
	}

	// Add Bob to group
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/groups/%s/members", srv.URL, groupResp.GroupID),
		map[string]string{"user_id": "bob2"}, aliceTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d", resp.StatusCode)
	}

	// Alice posts EVEN expense
	resp = doJSON(t, srv.Client(), "POST", srv.URL+"/expenses", map[string]any{
		"group_id":    groupResp.GroupID,
		"description": "dinner",
		"total_cents": 2000,
		"split_type":  "EQUAL",
		"splits": []map[string]any{
			{"user_id": "alice2"},
			{"user_id": "bob2"},
		},
	}, aliceTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create expense: expected 201, got %d", resp.StatusCode)
	}

	// GET /balances — should show Bob owes Alice
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/balances", nil, aliceTok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /balances: expected 200, got %d", resp.StatusCode)
	}
	var balanceResp struct {
		NetBalances          map[string]int64 `json:"net_balances"`
		SuggestedSettlements []any            `json:"suggested_settlements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&balanceResp); err != nil {
		t.Fatalf("decode balances: %v", err)
	}
	resp.Body.Close()
	if len(balanceResp.SuggestedSettlements) == 0 {
		t.Error("expected non-empty suggested_settlements after expense")
	}
}
