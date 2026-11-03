package identity

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
)

// Handler serves HTTP endpoints for identity and authentication.
type Handler struct {
	service Service
	cfg     *config.Config
}

// NewHandler instantiates a new Handler.
func NewHandler(service Service, cfg *config.Config) *Handler {
	return &Handler{
		service: service,
		cfg:     cfg,
	}
}

// RegisterRequest models the payload for candidate sign-up.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// LoginRequest models the payload for user authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse returns the authenticated user and anti-CSRF token.
type AuthResponse struct {
	User      *User     `json:"user"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RegisterRoutes mounts identity endpoints onto a chi.Router.
func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware, csrfMiddleware func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		// CSRF token retrieval endpoint (pre-auth or session-bound)
		r.Get("/csrf", h.HandleCSRF)

		// Public endpoints with rate limiting and strict Content-Type enforcement
		r.Post("/register", h.HandleRegister)
		r.Post("/login", h.HandleLogin)

		// Protected endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(RequireAuth)
			r.Use(csrfMiddleware)

			r.Post("/logout", h.HandleLogout)
			r.Get("/me", h.HandleMe)
			r.Get("/profile", h.HandleGetProfile)
			r.Post("/profile", h.HandleUpdateProfile)
		})
	})
}

// HandleRegister processes new candidate sign-ups.
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if !h.requireJSONContentType(w, r) {
		return
	}
	if !h.validatePreAuthCSRF(w, r) {
		return
	}

	// Guard against payload bomb (max 64 KB)
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, r, "Malformed request body: valid JSON required")
		return
	}

	ip := clientIP(r)
	userAgent := r.UserAgent()

	user, session, rawToken, csrfToken, err := h.service.Register(r.Context(), req.Email, req.Password, req.FullName, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateEmail):
			httperr.WriteJSON(w, r, http.StatusConflict, "Email Conflict", "An account with this email address already exists", "EMAIL_ALREADY_REGISTERED")
		case errors.Is(err, ErrRateLimited):
			httperr.WriteJSON(w, r, http.StatusTooManyRequests, "Rate Limited", err.Error(), "RATE_LIMITED")
		default:
			httperr.BadRequest(w, r, err.Error())
		}
		return
	}

	h.setAuthCookies(w, rawToken, csrfToken, session.ExpiresAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		User:      user,
		CSRFToken: csrfToken,
		ExpiresAt: session.ExpiresAt,
	})
}

// HandleLogin authenticates an existing user.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.requireJSONContentType(w, r) {
		return
	}
	if !h.validatePreAuthCSRF(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, r, "Malformed request body: valid JSON required")
		return
	}

	ip := clientIP(r)
	userAgent := r.UserAgent()

	user, session, rawToken, csrfToken, err := h.service.Login(r.Context(), req.Email, req.Password, ip, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Invalid email or password", "INVALID_CREDENTIALS")
		case errors.Is(err, ErrUserInactive):
			httperr.WriteJSON(w, r, http.StatusForbidden, "Account Inactive", "Your account is deactivated. Please contact support", "ACCOUNT_INACTIVE")
		case errors.Is(err, ErrRateLimited):
			httperr.WriteJSON(w, r, http.StatusTooManyRequests, "Rate Limited", err.Error(), "RATE_LIMITED")
		default:
			httperr.BadRequest(w, r, "Authentication failed")
		}
		return
	}

	h.setAuthCookies(w, rawToken, csrfToken, session.ExpiresAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		User:      user,
		CSRFToken: csrfToken,
		ExpiresAt: session.ExpiresAt,
	})
}

// HandleLogout invalidates the user's active session.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookieName := "session_token"
	domain := ""
	if h.cfg != nil {
		cookieName = h.cfg.Session.CookieName
		domain = h.cfg.Session.CookieDomain
	}

	var rawToken string
	cookie, err := r.Cookie(cookieName)
	if err == nil {
		rawToken = cookie.Value
	}

	ip := clientIP(r)
	_ = h.service.Logout(r.Context(), rawToken, ip, r.UserAgent())

	// Purge cookies in browser
	http.SetCookie(w, ExpireSessionCookie(cookieName, domain))
	http.SetCookie(w, ExpireSessionCookie(CookieCSRFName, domain))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out successfully",
	})
}

// HandleMe returns the currently authenticated user profile.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	session, _ := SessionFromContext(r.Context())

	csrfToken := ""
	csrfSecret := "super-secret-csrf-key-min-32-bytes!"
	if h.cfg != nil && h.cfg.Auth.CSRFSecret != "" {
		csrfSecret = h.cfg.Auth.CSRFSecret
	}
	if session != nil {
		csrfToken = GenerateSessionCSRFToken(session.SessionTokenHash, csrfSecret)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		User:      user,
		CSRFToken: csrfToken,
		ExpiresAt: session.ExpiresAt,
	})
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, rawToken, csrfToken string, expiresAt time.Time) {
	cookieName := "session_token"
	domain := ""
	secure := false
	if h.cfg != nil {
		cookieName = h.cfg.Session.CookieName
		domain = h.cfg.Session.CookieDomain
		secure = h.cfg.Session.CookieSecure
	}

	remainingDuration := time.Until(expiresAt)
	if remainingDuration <= 0 {
		remainingDuration = 336 * time.Hour
	}

	// 1. Session cookie (HttpOnly)
	http.SetCookie(w, CreateSessionCookie(cookieName, rawToken, remainingDuration, secure, domain))

	// 2. CSRF cookie (readable by SPA client)
	http.SetCookie(w, CreateCSRFCookie(csrfToken, remainingDuration, secure, domain))

	// 3. Response header for direct client extraction
	w.Header().Set(HeaderXCSRFToken, csrfToken)
}

func clientIP(r *http.Request) string {
	// If behind a trusted reverse proxy (like Caddy), check X-Real-IP or X-Forwarded-For
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fallback to RemoteAddr (strip port if present)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// HandleCSRF issues a CSRF token. If a session is active, it returns the session-bound token;
// otherwise it issues a cryptographically secure pre-auth CSRF token.
func (h *Handler) HandleCSRF(w http.ResponseWriter, r *http.Request) {
	domain := ""
	secure := false
	csrfSecret := "super-secret-csrf-key-min-32-bytes!"
	expiry := 336 * time.Hour
	if h.cfg != nil {
		domain = h.cfg.Session.CookieDomain
		secure = h.cfg.Session.CookieSecure
		csrfSecret = h.cfg.Auth.CSRFSecret
		expiry = h.cfg.Session.CookieExpiry
	}

	var csrfToken string
	session, ok := SessionFromContext(r.Context())
	if ok && session != nil {
		csrfToken = GenerateSessionCSRFToken(session.SessionTokenHash, csrfSecret)
	} else {
		if existing, err := r.Cookie(CookieCSRFName); err == nil && strings.TrimSpace(existing.Value) != "" {
			csrfToken = existing.Value
		} else {
			token, err := GeneratePreAuthCSRFToken()
			if err != nil {
				httperr.WriteJSON(w, r, http.StatusInternalServerError, "Internal Server Error", "Failed to generate CSRF token", "INTERNAL_SERVER_ERROR")
				return
			}
			csrfToken = token
		}
	}

	http.SetCookie(w, CreateCSRFCookie(csrfToken, expiry, secure, domain))
	w.Header().Set(HeaderXCSRFToken, csrfToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"csrf_token": csrfToken,
	})
}

func (h *Handler) requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "application/json") {
		httperr.WriteJSON(w, r, http.StatusUnsupportedMediaType, "Unsupported Media Type", "Content-Type must be application/json", "UNSUPPORTED_MEDIA_TYPE")
		return false
	}
	return true
}

func (h *Handler) validatePreAuthCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(CookieCSRFName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		headerToken := r.Header.Get(HeaderXCSRFToken)
		if headerToken == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
			httperr.WriteJSON(w, r, http.StatusForbidden, "CSRF Verification Failed", "Pre-authentication CSRF token mismatch", "INVALID_CSRF_TOKEN")
			return false
		}
	}
	return true
}

// HandleGetProfile returns the candidate preparation profile.
func (h *Handler) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	profile, err := h.service.GetProfile(r.Context(), user.ID)
	if err != nil {
		httperr.WriteJSON(w, r, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user profile", "INTERNAL_SERVER_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

// HandleUpdateProfile creates or updates the user profile and onboarding settings.
func (h *Handler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	if !h.requireJSONContentType(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req UserProfile
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, r, "Malformed request body: valid JSON required")
		return
	}

	req.UserID = user.ID
	profile, err := h.service.UpdateProfile(r.Context(), &req)
	if err != nil {
		slog.Default().Error("failed to update user profile", "error", err)
		httperr.WriteJSON(w, r, http.StatusInternalServerError, "Internal Server Error", "Failed to update user profile", "INTERNAL_SERVER_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}


