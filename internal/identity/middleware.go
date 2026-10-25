package identity

import (
	"context"
	"net/http"
	"strings"

	"github.com/adaptive-dsa/platform/internal/platform/httperr"
)

type userContextKey string
type sessionContextKey string

const (
	ContextKeyUser    userContextKey    = "authenticated_user"
	ContextKeySession sessionContextKey = "authenticated_session"
)

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) (*User, bool) {
	if ctx == nil {
		return nil, false
	}
	u, ok := ctx.Value(ContextKeyUser).(*User)
	return u, ok && u != nil
}

// SessionFromContext retrieves the authenticated session from the context.
func SessionFromContext(ctx context.Context) (*Session, bool) {
	if ctx == nil {
		return nil, false
	}
	s, ok := ctx.Value(ContextKeySession).(*Session)
	return s, ok && s != nil
}

// Authenticate extracts session tokens from cookies or Authorization header and attaches the user to context.
func Authenticate(svc Service, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var rawToken string

			// 1. Try reading from cookie
			cookie, err := r.Cookie(cookieName)
			if err == nil && strings.TrimSpace(cookie.Value) != "" {
				rawToken = cookie.Value
			} else {
				// 2. Fallback to Authorization: Bearer <token>
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					rawToken = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if rawToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			session, err := svc.ValidateSession(r.Context(), rawToken)
			if err != nil || session == nil || session.User == nil || !session.User.IsActive {
				next.ServeHTTP(w, r)
				return
			}

			// Attach session and user to context
			ctx := context.WithValue(r.Context(), ContextKeySession, session)
			ctx = context.WithValue(ctx, ContextKeyUser, session.User)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth blocks unauthenticated requests, returning RFC 9457 HTTP 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || user == nil {
			httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required to access this resource", "UNAUTHORIZED")
			return
		}

		if !user.IsActive {
			httperr.WriteJSON(w, r, http.StatusForbidden, "Forbidden", "User account has been deactivated", "ACCOUNT_DEACTIVATED")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole enforces specific role permissions (e.g., RoleAdmin), returning RFC 9457 HTTP 403 on mismatch.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || user == nil {
				httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
				return
			}

			if user.Role != role {
				httperr.WriteJSON(w, r, http.StatusForbidden, "Forbidden", "Insufficient administrative privileges", "FORBIDDEN")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireCSRF enforces anti-CSRF token verification on state-changing requests when a session is active.
func RequireCSRF(csrfSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Safe methods do not mutate state
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			// If no session exists in context (e.g. public login or register), allow through
			session, ok := SessionFromContext(r.Context())
			if !ok || session == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Session is active: candidate MUST provide valid X-CSRF-Token header
			candidateToken := r.Header.Get(HeaderXCSRFToken)
			if !VerifySessionCSRFToken(candidateToken, session.SessionTokenHash, csrfSecret) {
				httperr.WriteJSON(w, r, http.StatusForbidden, "CSRF Verification Failed", "A valid X-CSRF-Token header is required for state-changing requests", "INVALID_CSRF_TOKEN")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
