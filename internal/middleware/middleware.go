// Package middleware holds the HTTP middleware wrapped around every
// request.
package middleware

import "net/http"

// csp allows only the app's own scripts, styles and fonts. Inline styles
// are allowed for the style="" attributes templ renders (chart widths)
// and the indicator styles htmx injects. frame-ancestors stops other
// sites from framing the app (clickjacking).
const csp = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// SecurityHeaders sets browser security headers on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// CrossOrigin rejects unsafe requests (POST, DELETE, ...) sent by a
// browser from another site. Browsers re-send Basic Auth credentials on
// cross-site form posts, so without this any page the admin visits could
// write to the app. Requests without browser headers (curl, API clients)
// are let through.
func CrossOrigin(next http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(next)
}
