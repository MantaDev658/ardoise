package handlers

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
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/infrastructure/postgres"
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

	if err := postgres.RunMigrations(rawDB); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	for _, stmt := range []string{
		"DELETE FROM splits",
		"DELETE FROM expenses",
		"DELETE FROM group_invitations",
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
	invitationRepo := postgres.NewInvitationRepository(rawDB)

	userSvc := application.NewUserService(userRepo, secret)
	groupSvc := application.NewGroupService(groupRepo, expenseRepo, invitationRepo, userRepo, db)
	expenseSvc := application.NewExpenseService(expenseRepo, groupRepo, db)

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
	protected.HandleFunc("GET /invitations", h.ListMyInvitations)
	protected.HandleFunc("POST /invitations/{id}/accept", h.AcceptInvitation)
	protected.HandleFunc("POST /invitations/{id}/decline", h.DeclineInvitation)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", h.RegisterUser)
	mux.HandleFunc("POST /auth/login", h.LoginUser)
	mux.Handle("/", middleware.AuthMiddleware(auth)(protected))

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

// TestIntegration_GroupAndExpenseFlow exercises the full create-group → invite-member →
// accept-invite → post-expense → get-balances path against a real database.
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
	bobTok := register("bob2", "Bob")

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

	// Alice invites Bob — creates a pending invitation
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/groups/%s/members", srv.URL, groupResp.GroupID),
		map[string]string{"user_id": "bob2"}, aliceTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("invite member: expected 200, got %d", resp.StatusCode)
	}

	// Bob lists invitations and accepts
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/invitations", nil, bobTok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list invitations: expected 200, got %d", resp.StatusCode)
	}
	var invitations []struct {
		ID string `json:"ID"`
	}
	json.NewDecoder(resp.Body).Decode(&invitations) //nolint:errcheck
	resp.Body.Close()
	if len(invitations) == 0 {
		t.Fatal("expected at least one pending invitation for Bob")
	}

	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/invitations/%s/accept", srv.URL, invitations[0].ID),
		nil, bobTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("accept invitation: expected 200, got %d", resp.StatusCode)
	}

	// Alice posts EVEN expense — bob is now a member so this should succeed
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

// TestIntegration_InvitationDeclineAndErrors covers the decline path and the
// error cases (wrong actor → 403, missing invitation → 404) that the happy-path
// flow test does not exercise.
func TestIntegration_InvitationDeclineAndErrors(t *testing.T) {
	srv := setupIntegrationServer(t)

	register := func(id, displayName string) string {
		resp := doJSON(t, srv.Client(), "POST", srv.URL+"/auth/register", map[string]string{
			"id": id, "display_name": displayName, "password": "pass1234",
		}, "")
		resp.Body.Close()
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

	aliceTok := register("inv-alice", "Alice")
	bobTok := register("inv-bob", "Bob")
	charlieTok := register("inv-charlie", "Charlie")

	// Alice creates a group and invites Bob.
	resp := doJSON(t, srv.Client(), "POST", srv.URL+"/groups", map[string]string{"name": "Decline Test Group"}, aliceTok)
	var groupResp struct {
		GroupID string `json:"group_id"`
	}
	json.NewDecoder(resp.Body).Decode(&groupResp) //nolint:errcheck
	resp.Body.Close()

	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/groups/%s/members", srv.URL, groupResp.GroupID),
		map[string]string{"user_id": "inv-bob"}, aliceTok)
	resp.Body.Close()

	// Bob lists his invitations.
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/invitations", nil, bobTok)
	var invitations []struct {
		ID string `json:"ID"`
	}
	json.NewDecoder(resp.Body).Decode(&invitations) //nolint:errcheck
	resp.Body.Close()
	if len(invitations) == 0 {
		t.Fatal("expected at least one pending invitation for Bob")
	}
	invID := invitations[0].ID

	// Charlie (not the invitee) cannot accept Bob's invitation → 403.
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/invitations/%s/accept", srv.URL, invID), nil, charlieTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong actor accept: expected 403, got %d", resp.StatusCode)
	}

	// Charlie (not the invitee) cannot decline Bob's invitation → 403.
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/invitations/%s/decline", srv.URL, invID), nil, charlieTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong actor decline: expected 403, got %d", resp.StatusCode)
	}

	// Bob declines the invitation → 200.
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/invitations/%s/decline", srv.URL, invID), nil, bobTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("decline: expected 200, got %d", resp.StatusCode)
	}

	// Invitation is gone — accept on the now-deleted ID → 404.
	resp = doJSON(t, srv.Client(), "POST",
		fmt.Sprintf("%s/invitations/%s/accept", srv.URL, invID), nil, bobTok)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("accept after decline: expected 404, got %d", resp.StatusCode)
	}

	// Bob's invitation list should now be empty.
	resp = doJSON(t, srv.Client(), "GET", srv.URL+"/invitations", nil, bobTok)
	var remaining []struct {
		ID string `json:"ID"`
	}
	json.NewDecoder(resp.Body).Decode(&remaining) //nolint:errcheck
	resp.Body.Close()
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining invitations, got %d", len(remaining))
	}
}
