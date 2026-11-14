// Package auth protects the whole app with HTTP Basic Auth and two
// hardcoded (env-configured) users: an admin and a read-only viewer.
package auth

import (
	"context"
	"crypto/subtle"
	"net/http"

	"github.com/SaNog2/BeMovin/internal/config"
)

type Role int

const (
	RoleNone Role = iota
	RoleViewer
	RoleAdmin
)

type ctxKey struct{}

type user struct {
	name string
	role Role
}

// FromContext returns the role of the authenticated user.
func FromContext(ctx context.Context) Role {
	u, _ := ctx.Value(ctxKey{}).(user)
	return u.role
}

// UserName returns the login name of the authenticated user.
func UserName(ctx context.Context) string {
	u, _ := ctx.Value(ctxKey{}).(user)
	return u.name
}

// IsAdmin reports whether the request was made by the admin.
func IsAdmin(ctx context.Context) bool {
	return FromContext(ctx) == RoleAdmin
}

func eq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func authenticate(cfg *config.Config, r *http.Request) Role {
	u, p, ok := r.BasicAuth()
	if !ok {
		return RoleNone
	}
	switch {
	case eq(u, cfg.AdminUser) && eq(p, cfg.AdminPass):
		return RoleAdmin
	case eq(u, cfg.ViewerUser) && eq(p, cfg.ViewerPass):
		return RoleViewer
	}
	return RoleNone
}

// Require rejects unauthenticated requests and restricts the viewer to
// read-only methods, except for requests viewerWrite allows (it may be
// nil). The admin can use every method. The user's role is stored in the
// request context.
func Require(cfg *config.Config, next http.Handler, viewerWrite func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := authenticate(cfg, r)
		switch role {
		case RoleNone:
			w.Header().Set("WWW-Authenticate", `Basic realm="BeMovin", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		case RoleViewer:
			readOnly := r.Method == http.MethodGet || r.Method == http.MethodHead
			if !readOnly && (viewerWrite == nil || !viewerWrite(r)) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		name, _, _ := r.BasicAuth()
		ctx := context.WithValue(r.Context(), ctxKey{}, user{name: name, role: role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
