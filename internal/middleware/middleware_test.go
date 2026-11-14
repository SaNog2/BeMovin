package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	SecurityHeaders(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
}

func TestCrossOrigin(t *testing.T) {
	tests := []struct {
		name, method string
		headers      map[string]string
		want         int
	}{
		{"same-origin post", http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}, http.StatusOK},
		{"cross-site post", http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"cross-site post, origin only", http.MethodPost, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"cross-site get", http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusOK},
		{"api client post", http.MethodPost, nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://bemovin.example/trainings/new", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			CrossOrigin(ok).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
