package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ardoise/apps/backend/internal/core/application"
	hmacauth "ardoise/apps/backend/internal/core/infrastructure/auth/hmac"
	"ardoise/apps/backend/internal/core/infrastructure/http/handlers"
	"ardoise/apps/backend/internal/core/infrastructure/http/middleware"
	"ardoise/apps/backend/internal/core/infrastructure/postgres"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type config struct {
	dbURL      string
	jwtSecret  string
	corsOrigin string
	port       string
}

func loadConfig() config {
	_ = godotenv.Load()
	return config{
		dbURL:      mustEnv("DATABASE_URL"),
		jwtSecret:  mustEnv("JWT_SECRET"),
		corsOrigin: envOr("CORS_ORIGIN", "*"),
		port:       ":" + envOr("PORT", "8080"),
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s environment variable is required", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func spaHandler(dir string) http.Handler {
	fs := http.Dir(dir)
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("application failed: %v", err)
	}
}

func run() error {
	cfg := loadConfig()

	rawDB, err := sql.Open("postgres", cfg.dbURL)
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer rawDB.Close()

	rawDB.SetMaxOpenConns(25)
	rawDB.SetMaxIdleConns(5)
	rawDB.SetConnMaxLifetime(5 * time.Minute)

	if err := postgres.RunMigrations(rawDB); err != nil {
		return fmt.Errorf("migrations failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	postgres.StartPartitionManager(ctx, rawDB, 12)

	db := postgres.NewDB(rawDB)
	auditRepo := postgres.NewAuditRepository(rawDB)
	userRepo := postgres.NewUserRepository(rawDB)
	groupRepo := postgres.NewGroupRepository(rawDB)
	expenseRepo := postgres.NewExpenseRepository(rawDB)
	invitationRepo := postgres.NewInvitationRepository(rawDB)

	userService := application.NewUserService(userRepo, []byte(cfg.jwtSecret))
	groupService := application.NewGroupService(groupRepo, expenseRepo, auditRepo, invitationRepo, userRepo, db)
	expenseService := application.NewExpenseService(expenseRepo, groupRepo, auditRepo, db)

	h := handlers.NewAPIHandler(expenseService, userService, groupService)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /api/expenses", h.CreateExpense)
	protected.HandleFunc("GET /api/expenses", h.ListExpenses)
	protected.HandleFunc("GET /api/balances", h.GetBalances)
	protected.HandleFunc("GET /api/friends/{user_id}/balances", h.GetFriendBalances)
	protected.HandleFunc("PUT /api/expenses/{id}", h.UpdateExpense)
	protected.HandleFunc("DELETE /api/expenses/{id}", h.DeleteExpense)
	protected.HandleFunc("POST /api/settlements", h.CreateSettlement)

	protected.HandleFunc("GET /api/users/me", h.GetCurrentUser)
	protected.HandleFunc("GET /api/friends", h.ListFriends)
	protected.HandleFunc("GET /api/users", h.ListUsers)
	protected.HandleFunc("PUT /api/users/{id}", h.UpdateUser)
	protected.HandleFunc("PUT /api/users/{id}/password", h.ChangePassword)
	protected.HandleFunc("DELETE /api/users/{id}", h.DeleteUser)

	protected.HandleFunc("POST /api/groups", h.CreateGroup)
	protected.HandleFunc("GET /api/groups", h.ListGroups)
	protected.HandleFunc("PUT /api/groups/{id}", h.UpdateGroup)
	protected.HandleFunc("DELETE /api/groups/{id}", h.DeleteGroup)
	protected.HandleFunc("POST /api/groups/{id}/members", h.AddGroupMember)
	protected.HandleFunc("DELETE /api/groups/{id}/members/{user_id}", h.RemoveGroupMember)
	protected.HandleFunc("GET /api/groups/{id}/activity", h.GetGroupActivity)

	protected.HandleFunc("GET /api/invitations", h.ListMyInvitations)
	protected.HandleFunc("POST /api/invitations/{id}/accept", h.AcceptInvitation)
	protected.HandleFunc("POST /api/invitations/{id}/decline", h.DeclineInvitation)

	auth := hmacauth.New([]byte(cfg.jwtSecret))
	authLimiter := middleware.NewRateLimiter(10, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.Handle("POST /api/auth/register", authLimiter.Middleware(http.HandlerFunc(h.RegisterUser)))
	mux.Handle("POST /api/auth/login", authLimiter.Middleware(http.HandlerFunc(h.LoginUser)))
	mux.Handle("/api/", middleware.AuthMiddleware(auth)(protected))

	webDir := envOr("WEB_DIR", "/web")
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		mux.Handle("/", spaHandler(webDir))
	}

	handler := middleware.SecurityHeaders(
		middleware.CORSMiddleware(cfg.corsOrigin)(
			http.TimeoutHandler(mux, 10*time.Second, `{"error":"request timeout"}`),
		),
	)

	server := &http.Server{
		Addr:              cfg.port,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	fmt.Printf("API running on %s\n", cfg.port)
	serverError := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverError:
		return fmt.Errorf("server crashed: %w", err)
	case <-stop:
		log.Println("Shutting down gracefully...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	log.Println("server stopped")
	return nil
}
