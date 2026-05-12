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
	"syscall"
	"time"

	"opensplit/apps/backend/internal/core/application"
	openhttp "opensplit/apps/backend/internal/core/infrastructure/http"
	"opensplit/apps/backend/internal/core/infrastructure/postgres"

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

	db := postgres.NewDB(rawDB)
	auditRepo := postgres.NewAuditRepository(rawDB)
	userRepo := postgres.NewUserRepository(rawDB)
	groupRepo := postgres.NewGroupRepository(rawDB)
	expenseRepo := postgres.NewExpenseRepository(rawDB)

	userService := application.NewUserService(userRepo, []byte(cfg.jwtSecret))
	groupService := application.NewGroupService(groupRepo, expenseRepo, auditRepo, db)
	expenseService := application.NewExpenseService(expenseRepo, groupRepo, auditRepo, db)

	h := openhttp.NewAPIHandler(expenseService, userService, groupService)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /expenses", h.CreateExpense)
	protected.HandleFunc("GET /expenses", h.ListExpenses)
	protected.HandleFunc("GET /balances", h.GetBalances)
	protected.HandleFunc("GET /friends/{user_id}/balances", h.GetFriendBalances)
	protected.HandleFunc("PUT /expenses/{id}", h.UpdateExpense)
	protected.HandleFunc("DELETE /expenses/{id}", h.DeleteExpense)
	protected.HandleFunc("POST /settlements", h.CreateSettlement)

	protected.HandleFunc("GET /friends", h.ListFriends)
	protected.HandleFunc("GET /users", h.ListUsers)
	protected.HandleFunc("PUT /users/{id}", h.UpdateUser)
	protected.HandleFunc("PUT /users/{id}/password", h.ChangePassword)
	protected.HandleFunc("DELETE /users/{id}", h.DeleteUser)

	protected.HandleFunc("POST /groups", h.CreateGroup)
	protected.HandleFunc("GET /groups", h.ListGroups)
	protected.HandleFunc("PUT /groups/{id}", h.UpdateGroup)
	protected.HandleFunc("DELETE /groups/{id}", h.DeleteGroup)
	protected.HandleFunc("POST /groups/{id}/members", h.AddGroupMember)
	protected.HandleFunc("DELETE /groups/{id}/members/{user_id}", h.RemoveGroupMember)
	protected.HandleFunc("GET /groups/{id}/activity", h.GetGroupActivity)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", h.RegisterUser)
	mux.HandleFunc("POST /auth/login", h.LoginUser)
	mux.Handle("/", openhttp.AuthMiddleware([]byte(cfg.jwtSecret))(protected))

	handler := openhttp.CORSMiddleware(cfg.corsOrigin)(
		http.TimeoutHandler(mux, 10*time.Second, `{"error":"request timeout"}`),
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	log.Println("server stopped")
	return nil
}
