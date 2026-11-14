package auth

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/SaNog2/BeMovin/internal/config"
)

func TestRequireAuth(t *testing.T) {
	cfg := &config.Config{AdminUser: "admin", AdminPass: "a-pass", ViewerUser: "viewer", ViewerPass: "v-pass"}
	h := Require(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Role", strconv.Itoa(int(FromContext(r.Context()))))
		w.WriteHeader(http.StatusOK)
	}), func(r *http.Request) bool { return r.Method == http.MethodPost && r.URL.Path == "/open" })

	tests := []struct {
		name, method, path, user, pass string
		noAuth                         bool
		want                           int
		wantRole                       Role
	}{
		{"no credentials", http.MethodGet, "/", "", "", true, http.StatusUnauthorized, RoleNone},
		{"wrong password", http.MethodGet, "/", "admin", "nope", false, http.StatusUnauthorized, RoleNone},
		{"viewer password for admin", http.MethodGet, "/", "admin", "v-pass", false, http.StatusUnauthorized, RoleNone},
		{"admin GET", http.MethodGet, "/", "admin", "a-pass", false, http.StatusOK, RoleAdmin},
		{"admin POST", http.MethodPost, "/", "admin", "a-pass", false, http.StatusOK, RoleAdmin},
		{"admin DELETE", http.MethodDelete, "/", "admin", "a-pass", false, http.StatusOK, RoleAdmin},
		{"viewer GET", http.MethodGet, "/", "viewer", "v-pass", false, http.StatusOK, RoleViewer},
		{"viewer POST", http.MethodPost, "/", "viewer", "v-pass", false, http.StatusForbidden, RoleNone},
		{"viewer DELETE", http.MethodDelete, "/", "viewer", "v-pass", false, http.StatusForbidden, RoleNone},
		{"viewer POST allowed path", http.MethodPost, "/open", "viewer", "v-pass", false, http.StatusOK, RoleViewer},
		{"viewer DELETE allowed path", http.MethodDelete, "/open", "viewer", "v-pass", false, http.StatusForbidden, RoleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if !tt.noAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
			if tt.wantRole != RoleNone && rec.Header().Get("X-Role") != strconv.Itoa(int(tt.wantRole)) {
				t.Fatalf("got role %s, want %d", rec.Header().Get("X-Role"), tt.wantRole)
			}
			if tt.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing WWW-Authenticate header")
			}
		})
	}
}
