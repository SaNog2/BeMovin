package main

import (
	"crypto/subtle"
	"net/http"

	"github.com/SaNog2/BeMovin/internal/config"
)

type role int

const (
	roleNone role = iota
	roleViewer
	roleAdmin
)

func eq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func authenticate(cfg *config.Config, r *http.Request) role {
	u, p, ok := r.BasicAuth()
	if !ok {
		return roleNone
	}
	switch {
	case eq(u, cfg.AdminUser) && eq(p, cfg.AdminPass):
		return roleAdmin
	case eq(u, cfg.ViewerUser) && eq(p, cfg.ViewerPass):
		return roleViewer
	}
	return roleNone
}

// requireAuth rejects unauthenticated requests and restricts the viewer
// to read-only methods. The admin can use every method.
func requireAuth(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch authenticate(cfg, r) {
		case roleNone:
			w.Header().Set("WWW-Authenticate", `Basic realm="BeMovin", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		case roleViewer:
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
