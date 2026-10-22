package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestValidateCreate(t *testing.T) {
	cases := []struct {
		name      string
		in        taskInput
		wantError bool
	}{
		{"valid", taskInput{Title: "Ship it", Status: "todo"}, false},
		{"valid without status", taskInput{Title: "Ship it"}, false},
		{"missing title", taskInput{Status: "todo"}, true},
		{"blank title", taskInput{Title: "   "}, true},
		{"invalid status", taskInput{Title: "Ship it", Status: "blocked"}, true},
		{"title too long", taskInput{Title: strings.Repeat("a", 201)}, true},
		{"title at limit counts characters", taskInput{Title: strings.Repeat("é", 200)}, false},
		{"description too long", taskInput{Title: "x", Description: strings.Repeat("a", 5001)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateCreate(tc.in) != ""
			if got != tc.wantError {
				t.Fatalf("validateCreate() error = %v, want %v", got, tc.wantError)
			}
		})
	}
}

func TestValidatePatch(t *testing.T) {
	cases := []struct {
		name      string
		in        taskPatch
		wantError bool
	}{
		{"status only", taskPatch{Status: strPtr("done")}, false},
		{"clear description", taskPatch{Description: strPtr("")}, false},
		{"empty patch", taskPatch{}, true},
		{"blank title", taskPatch{Title: strPtr("  ")}, true},
		{"bad status", taskPatch{Status: strPtr("blocked")}, true},
		{"empty status", taskPatch{Status: strPtr("")}, true},
		{"title too long", taskPatch{Title: strPtr(strings.Repeat("a", 201))}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validatePatch(tc.in) != ""
			if got != tc.wantError {
				t.Fatalf("validatePatch() error = %v, want %v", got, tc.wantError)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	if got := normalizeStatus(""); got != "todo" {
		t.Fatalf("got %q, want todo", got)
	}
	if got := normalizeStatus("done"); got != "done" {
		t.Fatalf("got %q, want done", got)
	}
}

func TestTrimPtr(t *testing.T) {
	if trimPtr(nil) != nil {
		t.Fatal("nil must stay nil so the column is left unchanged")
	}
	if got := trimPtr(strPtr("  hi  ")); got == nil || *got != "hi" {
		t.Fatalf("unexpected trim result %v", got)
	}
	if got := trimPtr(strPtr("")); got == nil || *got != "" {
		t.Fatal("empty string must stay non-nil so a description can be cleared")
	}
}

func TestDecodeInput(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"valid", `{"title":"a"}`, true},
		{"unknown field", `{"title":"a","extra":1}`, false},
		{"malformed", `{"title":`, false},
		{"trailing data", `{"title":"a"} {"title":"b"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			var in taskInput
			if got := decodeInput(rec, req, &in); got != tc.ok {
				t.Fatalf("decodeInput() = %v, want %v", got, tc.ok)
			}
			if !tc.ok && rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	for raw, want := range map[string]bool{"1": true, "42": true, "0": false, "-1": false, "abc": false, "": false} {
		rec := httptest.NewRecorder()
		if _, ok := parseID(rec, raw); ok != want {
			t.Fatalf("parseID(%q) ok = %v, want %v", raw, ok, want)
		}
	}
}

func TestHealthURL(t *testing.T) {
	cases := map[string]string{
		":8080":          "http://127.0.0.1:8080/health/live",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/health/live",
		"127.0.0.1:8081": "http://127.0.0.1:8081/health/live",
		"garbage":        "http://127.0.0.1:8080/health/live",
	}
	for in, want := range cases {
		if got := healthURL(in); got != want {
			t.Fatalf("healthURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCORS(t *testing.T) {
	a := &app{cfg: config{allowedOrigin: map[string]bool{"http://localhost:8080": true}}}
	h := a.cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	h.ServeHTTP(allowed, req)
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8080" {
		t.Fatalf("allowed origin header = %q", got)
	}

	denied := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.example")
	h.ServeHTTP(denied, req)
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin must not be echoed, got %q", got)
	}

	preflight := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/", nil)
	h.ServeHTTP(preflight, req)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", preflight.Code)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error when DATABASE_URL is unset")
	}

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("DB_MAX_CONNS", "500")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error for DB_MAX_CONNS out of range")
	}

	t.Setenv("DB_MAX_CONNS", "5")
	t.Setenv("LOG_LEVEL", "verbose")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL")
	}

	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a.example, http://b.example")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.maxConns != 5 || !cfg.allowedOrigin["http://a.example"] || !cfg.allowedOrigin["http://b.example"] {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
