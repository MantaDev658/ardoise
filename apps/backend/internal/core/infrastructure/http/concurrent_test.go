package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	hmacauth "ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	"ardoise/apps/backend/internal/core/mocks"
	sharedjwt "ardoise/libs/shared/jwt"
)

// makeConcurrentTestServer builds a live httptest.Server backed by the full handler
// stack — real AuthMiddleware, real JWT validation — so the race detector exercises
// the actual request path under concurrent load.
func makeConcurrentTestServer(t *testing.T) (server *httptest.Server, bearerToken string) {
	t.Helper()
	const secret = "test-secret"

	es, us, gs := newTestServices(
		&mocks.MockExpenseRepo{},
		&mocks.MockUserRepo{},
		&mocks.MockGroupRepo{},
		&mocks.MockAuditRepo{},
	)
	h := NewAPIHandler(es, us, gs)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /expenses", h.ListExpenses)
	protected.HandleFunc("GET /groups", h.ListGroups)

	mux := http.NewServeMux()
	mux.Handle("/", AuthMiddleware(hmacauth.New([]byte(secret)))(protected))

	tok, err := sharedjwt.Sign("Alice", []byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	return httptest.NewServer(mux), tok
}

// TestAPIHandler_ConcurrentRequests fires 50 parallel requests through a live HTTP
// server so the race detector has real concurrent execution to inspect. Any shared
// handler state introduced in the future will be caught here.
func TestAPIHandler_ConcurrentRequests(t *testing.T) {
	server, tok := makeConcurrentTestServer(t)
	defer server.Close()

	endpoints := []string{"/expenses", "/groups"}
	const N = 50

	errs := make([]error, N)
	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			url := server.URL + endpoints[idx%len(endpoints)]
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				errs[idx] = err
				return
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := server.Client().Do(req)
			if err != nil {
				errs[idx] = err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs[idx] = fmt.Errorf("%s: unexpected status %d", url, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}
