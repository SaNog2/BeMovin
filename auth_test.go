package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SaNog2/BeMovin/internal/config"
)

func TestRequireAuth(t *testing.T) {
	cfg := &config.Config{AdminUser: "admin", AdminPass: "a-pass", ViewerUser: "viewer", ViewerPass: "v-pass"}
	h := requireAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name, method, user, pass string
		noAuth                   bool
		want                     int
	}{
		{"no credentials", http.MethodGet, "", "", true, http.StatusUnauthorized},
		{"wrong password", http.MethodGet, "admin", "nope", false, http.StatusUnauthorized},
		{"viewer password for admin", http.MethodGet, "admin", "v-pass", false, http.StatusUnauthorized},
		{"admin GET", http.MethodGet, "admin", "a-pass", false, http.StatusOK},
		{"admin POST", http.MethodPost, "admin", "a-pass", false, http.StatusOK},
		{"admin DELETE", http.MethodDelete, "admin", "a-pass", false, http.StatusOK},
		{"viewer GET", http.MethodGet, "viewer", "v-pass", false, http.StatusOK},
		{"viewer POST", http.MethodPost, "viewer", "v-pass", false, http.StatusForbidden},
		{"viewer DELETE", http.MethodDelete, "viewer", "v-pass", false, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			if !tt.noAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing WWW-Authenticate header")
			}
		})
	}
}
