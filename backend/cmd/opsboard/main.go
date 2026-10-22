package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
	addr          string
	databaseURL   string
	maxConns      int32
	allowedOrigin map[string]bool
	logLevel      slog.Level
}

type task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// taskInput is the request body for creating a task.
type taskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// taskPatch is the request body for a partial update. A nil field means
// "leave unchanged", which lets clients clear a description with "".
type taskPatch struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

const (
	maxTitleLen       = 200
	maxDescriptionLen = 5000
	defaultStatus     = "todo"
)

var validStatuses = map[string]bool{"todo": true, "in_progress": true, "done": true}

type app struct {
	db     *pgxpool.Pool
	logger *slog.Logger
	cfg    config
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.logLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("invalid database configuration: %w", err)
	}
	poolCfg.MaxConns = cfg.maxConns

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(startCtx, poolCfg)
	if err != nil {
		return fmt.Errorf("create database pool failed: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startCtx); err != nil {
		return fmt.Errorf("database is unavailable: %w", err)
	}

	a := &app{db: pool, logger: logger, cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", a.live)
	mux.HandleFunc("GET /health/ready", a.ready)
	mux.HandleFunc("GET /api/v1/tasks", a.tasks)
	mux.HandleFunc("POST /api/v1/tasks", a.createTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.taskByID)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", a.updateTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", a.deleteTask)

	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           a.logging(a.cors(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", cfg.addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

// runHealthcheck probes the local liveness endpoint. It backs the container
// HEALTHCHECK, because the distroless image has no curl or shell.
func runHealthcheck() int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(healthURL(env("HTTP_ADDR", ":8080")))
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func healthURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:8080/health/live"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/health/live"
}

func loadConfig() (config, error) {
	cfg := config{
		addr:        env("HTTP_ADDR", ":8080"),
		databaseURL: os.Getenv("DATABASE_URL"),
		maxConns:    10,
		logLevel:    slog.LevelInfo,
		allowedOrigin: map[string]bool{
			"http://localhost:8080": true,
		},
	}
	if cfg.databaseURL == "" {
		return cfg, errors.New("DATABASE_URL must be set")
	}
	if raw := os.Getenv("DB_MAX_CONNS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return cfg, errors.New("DB_MAX_CONNS must be between 1 and 100")
		}
		cfg.maxConns = int32(n)
	}
	if raw := os.Getenv("CORS_ALLOWED_ORIGINS"); raw != "" {
		cfg.allowedOrigin = map[string]bool{}
		for _, origin := range strings.Split(raw, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				cfg.allowedOrigin[origin] = true
			}
		}
	}
	switch strings.ToLower(env("LOG_LEVEL", "info")) {
	case "debug":
		cfg.logLevel = slog.LevelDebug
	case "info":
		cfg.logLevel = slog.LevelInfo
	case "warn":
		cfg.logLevel = slog.LevelWarn
	case "error":
		cfg.logLevel = slog.LevelError
	default:
		return cfg, errors.New("LOG_LEVEL must be debug, info, warn, or error")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (a *app) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (a *app) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *app) tasks(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(r.Context(), `SELECT id, title, description, status, created_at, updated_at FROM tasks ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		a.internalError(w, "list tasks failed", err)
		return
	}
	defer rows.Close()
	items := []task{}
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			a.internalError(w, "read task failed", err)
			return
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, "iterate tasks failed", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *app) createTask(w http.ResponseWriter, r *http.Request) {
	var in taskInput
	if !decodeInput(w, r, &in) {
		return
	}
	if msg := validateCreate(in); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	var t task
	err := a.db.QueryRow(r.Context(), `
		INSERT INTO tasks(title, description, status)
		VALUES($1, $2, $3)
		RETURNING id, title, description, status, created_at, updated_at`,
		strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), normalizeStatus(in.Status)).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		a.internalError(w, "create task failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (a *app) taskByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	t, err := getTask(r.Context(), a.db, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		a.internalError(w, "get task failed", err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *app) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var in taskPatch
	if !decodeInput(w, r, &in) {
		return
	}
	if msg := validatePatch(in); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	var t task
	err := a.db.QueryRow(r.Context(), `
		UPDATE tasks SET
		  title = COALESCE($2::text, title),
		  description = COALESCE($3::text, description),
		  status = COALESCE($4::text, status),
		  updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, description, status, created_at, updated_at`,
		id, trimPtr(in.Title), trimPtr(in.Description), in.Status).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		a.internalError(w, "update task failed", err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *app) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	tag, err := a.db.Exec(r.Context(), `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		a.internalError(w, "delete task failed", err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getTask(ctx context.Context, db *pgxpool.Pool, id int64) (task, error) {
	var t task
	err := db.QueryRow(ctx, `SELECT id, title, description, status, created_at, updated_at FROM tasks WHERE id = $1`, id).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func validateCreate(in taskInput) string {
	title := strings.TrimSpace(in.Title)
	switch {
	case title == "":
		return "title is required"
	case utf8.RuneCountInString(title) > maxTitleLen:
		return "title must be 200 characters or fewer"
	case utf8.RuneCountInString(in.Description) > maxDescriptionLen:
		return "description must be 5000 characters or fewer"
	case in.Status != "" && !validStatuses[in.Status]:
		return "status must be todo, in_progress, or done"
	}
	return ""
}

func validatePatch(in taskPatch) string {
	if in.Title == nil && in.Description == nil && in.Status == nil {
		return "at least one field is required"
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return "title cannot be empty"
		}
		if utf8.RuneCountInString(title) > maxTitleLen {
			return "title must be 200 characters or fewer"
		}
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > maxDescriptionLen {
		return "description must be 5000 characters or fewer"
	}
	if in.Status != nil && !validStatuses[*in.Status] {
		return "status must be todo, in_progress, or done"
	}
	return ""
}

func normalizeStatus(status string) string {
	if status == "" {
		return defaultStatus
	}
	return status
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

func decodeInput(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid task ID"})
		return 0, false
	}
	return id, true
}

func (a *app) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && a.cfg.allowedOrigin[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *app) internalError(w http.ResponseWriter, message string, err error) {
	a.logger.Error(message, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
