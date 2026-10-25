package identity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInMemoryRepository() *InMemoryRepository {
	return NewInMemoryRepository()
}

func testIdentitySetup() (*IdentityService, *Handler, *InMemoryRepository, *config.Config, chi.Router) {
	repo := newInMemoryRepository()
	cfg := &config.Config{
		AppEnv: config.EnvTest,
		Server: config.ServerConfig{Port: 8080},
		Session: config.SessionConfig{
			CookieName:   "test_session",
			CookieExpiry: 24 * time.Hour,
			CookieSecure: false,
		},
		Auth: config.AuthConfig{
			CSRFSecret:       "test-csrf-secret-key-at-least-32-bytes!",
			RateLimitAuthRPM: 5,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(repo, cfg, logger)
	handler := NewHandler(svc, cfg)

	r := chi.NewRouter()
	authMw := Authenticate(svc, cfg.Session.CookieName)
	csrfMw := RequireCSRF(cfg.Auth.CSRFSecret)
	handler.RegisterRoutes(r, authMw, csrfMw)

	// Admin-only test endpoint
	r.Route("/admin", func(r chi.Router) {
		r.Use(authMw)
		r.Use(RequireAuth)
		r.Use(RequireRole(RoleAdmin))
		r.Get("/secret", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"admin_ok"}`))
		})
	})

	return svc, handler, repo, cfg, r
}

func jsonReq(method, path, body string) *http.Request {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestRegistration_Success(t *testing.T) {
	_, _, repo, _, router := testIdentitySetup()

	payload := `{"email":"Candidate@Example.COM","password":"ValidPassword123!","full_name":"Jane Doe"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var authResp AuthResponse
	err := json.NewDecoder(resp.Body).Decode(&authResp)
	require.NoError(t, err)

	assert.Equal(t, "candidate@example.com", authResp.User.Email, "email must be normalized lowercase")
	assert.Equal(t, "Jane Doe", authResp.User.FullName)
	assert.Equal(t, RoleCandidate, authResp.User.Role, "registration must strictly default to CANDIDATE")
	assert.NotEmpty(t, authResp.CSRFToken)

	// Check cookies set
	cookies := resp.Cookies()
	require.NotEmpty(t, cookies)
	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "test_session" {
			sessionCookie = c
		}
		if c.Name == CookieCSRFName {
			csrfCookie = c
		}
	}
	require.NotNil(t, sessionCookie)
	assert.True(t, sessionCookie.HttpOnly)
	assert.NotEmpty(t, sessionCookie.Value)

	require.NotNil(t, csrfCookie)
	assert.False(t, csrfCookie.HttpOnly)

	// Check audit log
	assert.Len(t, repo.auditLogs, 1)
	assert.Equal(t, "USER_REGISTER", repo.auditLogs[0].Action)
}

func TestRegistration_DuplicateEmail_Rejected(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	payload := `{"email":"user@example.com","password":"Password123!","full_name":"First User"}`
	req1 := jsonReq(http.MethodPost, "/auth/register", payload)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Duplicate attempt with different casing
	payloadDup := `{"email":"USER@example.COM","password":"Password123!","full_name":"Duplicate User"}`
	req2 := jsonReq(http.MethodPost, "/auth/register", payloadDup)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), "EMAIL_ALREADY_REGISTERED")
}

func TestRegistration_WeakPassword_Rejected(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	payload := `{"email":"user@example.com","password":"123","full_name":"User"}`
	req := jsonReq(http.MethodPost, "/auth/register", payload)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "password must be at least 8 characters")
}

func TestLogin_Success_And_SessionRotation(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// 1. Register user
	regPayload := `{"email":"testlogin@example.com","password":"SecretPassword123!","full_name":"Login Tester"}`
	wReg := httptest.NewRecorder()
	router.ServeHTTP(wReg, jsonReq(http.MethodPost, "/auth/register", regPayload))
	assert.Equal(t, http.StatusCreated, wReg.Code)

	var regResp AuthResponse
	_ = json.NewDecoder(wReg.Body).Decode(&regResp)
	regSessionToken := ""
	for _, c := range wReg.Result().Cookies() {
		if c.Name == "test_session" {
			regSessionToken = c.Value
		}
	}

	// 2. Login
	loginPayload := `{"email":"TESTLOGIN@example.com","password":"SecretPassword123!"}`
	wLogin := httptest.NewRecorder()
	router.ServeHTTP(wLogin, jsonReq(http.MethodPost, "/auth/login", loginPayload))

	assert.Equal(t, http.StatusOK, wLogin.Code)
	var loginResp AuthResponse
	err := json.NewDecoder(wLogin.Body).Decode(&loginResp)
	require.NoError(t, err)

	assert.Equal(t, "testlogin@example.com", loginResp.User.Email)
	assert.NotEmpty(t, loginResp.CSRFToken)

	// Verify session was rotated (issued new token)
	loginSessionToken := ""
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "test_session" {
			loginSessionToken = c.Value
		}
	}
	assert.NotEmpty(t, loginSessionToken)
	assert.NotEqual(t, regSessionToken, loginSessionToken, "login must rotate session token")
}

func TestLogin_InvalidPassword_ReturnsGenericError(t *testing.T) {
	_, _, repo, _, router := testIdentitySetup()

	// Register
	regPayload := `{"email":"user@example.com","password":"CorrectPassword123!","full_name":"User"}`
	router.ServeHTTP(httptest.NewRecorder(), jsonReq(http.MethodPost, "/auth/register", regPayload))

	// Login with incorrect password
	loginPayload := `{"email":"user@example.com","password":"WrongPassword!"}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonReq(http.MethodPost, "/auth/login", loginPayload))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")

	// Audit check
	lastAudit := repo.auditLogs[len(repo.auditLogs)-1]
	assert.Equal(t, "USER_LOGIN_FAILED", lastAudit.Action)
}

func TestLogin_UnknownUser_ReturnsGenericError(t *testing.T) {
	_, _, repo, _, router := testIdentitySetup()

	loginPayload := `{"email":"ghost@example.com","password":"Password123!"}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonReq(http.MethodPost, "/auth/login", loginPayload))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CREDENTIALS")

	lastAudit := repo.auditLogs[len(repo.auditLogs)-1]
	assert.Equal(t, "USER_LOGIN_FAILED", lastAudit.Action)
}

func TestLogout_InvalidatesSession(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// 1. Register to obtain session and csrf
	regPayload := `{"email":"logout_test@example.com","password":"Password123!","full_name":"Logout Tester"}`
	wReg := httptest.NewRecorder()
	router.ServeHTTP(wReg, jsonReq(http.MethodPost, "/auth/register", regPayload))

	var sessionCookie *http.Cookie
	var csrfToken string
	for _, c := range wReg.Result().Cookies() {
		if c.Name == "test_session" {
			sessionCookie = c
		}
		if c.Name == CookieCSRFName {
			csrfToken = c.Value
		}
	}

	// 2. Logout with CSRF token header
	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutReq.Header.Set(HeaderXCSRFToken, csrfToken)
	wLogout := httptest.NewRecorder()

	router.ServeHTTP(wLogout, logoutReq)
	assert.Equal(t, http.StatusOK, wLogout.Code)

	// Verify cookie expiration header
	var expiredSessionCookie *http.Cookie
	for _, c := range wLogout.Result().Cookies() {
		if c.Name == "test_session" {
			expiredSessionCookie = c
		}
	}
	require.NotNil(t, expiredSessionCookie)
	assert.Equal(t, -1, expiredSessionCookie.MaxAge)

	// 3. Subsequent /auth/me call with old cookie must fail (401)
	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	wMe := httptest.NewRecorder()
	router.ServeHTTP(wMe, meReq)

	assert.Equal(t, http.StatusUnauthorized, wMe.Code)
}

func TestCSRFProtection_OnStateChangingEndpoints(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// Register user
	regPayload := `{"email":"csrf_victim@example.com","password":"Password123!","full_name":"Victim"}`
	wReg := httptest.NewRecorder()
	router.ServeHTTP(wReg, jsonReq(http.MethodPost, "/auth/register", regPayload))

	var sessionCookie *http.Cookie
	var validCSRFToken string
	for _, c := range wReg.Result().Cookies() {
		if c.Name == "test_session" {
			sessionCookie = c
		}
		if c.Name == CookieCSRFName {
			validCSRFToken = c.Value
		}
	}

	// A. State-changing POST without CSRF header must be REJECTED (403)
	reqNoCSRF := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	reqNoCSRF.AddCookie(sessionCookie)
	wNoCSRF := httptest.NewRecorder()
	router.ServeHTTP(wNoCSRF, reqNoCSRF)

	assert.Equal(t, http.StatusForbidden, wNoCSRF.Code)
	assert.Contains(t, wNoCSRF.Body.String(), "INVALID_CSRF_TOKEN")

	// B. State-changing POST with INVALID CSRF header must be REJECTED (403)
	reqBadCSRF := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	reqBadCSRF.AddCookie(sessionCookie)
	reqBadCSRF.Header.Set(HeaderXCSRFToken, "attacker-forged-token-xyz")
	wBadCSRF := httptest.NewRecorder()
	router.ServeHTTP(wBadCSRF, reqBadCSRF)

	assert.Equal(t, http.StatusForbidden, wBadCSRF.Code)
	assert.Contains(t, wBadCSRF.Body.String(), "INVALID_CSRF_TOKEN")

	// C. State-changing POST with VALID CSRF header must SUCCEED (200)
	reqGoodCSRF := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	reqGoodCSRF.AddCookie(sessionCookie)
	reqGoodCSRF.Header.Set(HeaderXCSRFToken, validCSRFToken)
	wGoodCSRF := httptest.NewRecorder()
	router.ServeHTTP(wGoodCSRF, reqGoodCSRF)

	assert.Equal(t, http.StatusOK, wGoodCSRF.Code)
}

func TestAdminProvisioning_And_RBACBoundary(t *testing.T) {
	svc, _, repo, _, router := testIdentitySetup()
	ctx := context.Background()

	// 1. Provision Administrator via CLI service
	adminUser, err := svc.CreateAdmin(ctx, "admin@platform.com", "AdminPassword123!", "Lead Admin", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, adminUser.Role)

	// Verify admin provisioning audit log
	lastAudit := repo.auditLogs[len(repo.auditLogs)-1]
	assert.Equal(t, "ADMIN_USER_PROVISIONED", lastAudit.Action)
	assert.Equal(t, adminUser.ID, *lastAudit.TargetID)

	// 2. Candidate login session
	regPayload := `{"email":"candidate@platform.com","password":"Password123!","full_name":"Candidate"}`
	wCandidate := httptest.NewRecorder()
	router.ServeHTTP(wCandidate, jsonReq(http.MethodPost, "/auth/register", regPayload))
	var candidateCookie *http.Cookie
	for _, c := range wCandidate.Result().Cookies() {
		if c.Name == "test_session" {
			candidateCookie = c
		}
	}

	// 3. Admin login session
	loginPayload := `{"email":"admin@platform.com","password":"AdminPassword123!"}`
	wAdmin := httptest.NewRecorder()
	router.ServeHTTP(wAdmin, jsonReq(http.MethodPost, "/auth/login", loginPayload))
	var adminCookie *http.Cookie
	for _, c := range wAdmin.Result().Cookies() {
		if c.Name == "test_session" {
			adminCookie = c
		}
	}

	// 4. Test RBAC endpoint: Candidate access must be FORBIDDEN (403)
	reqCandidate := httptest.NewRequest(http.MethodGet, "/admin/secret", nil)
	reqCandidate.AddCookie(candidateCookie)
	wCandResp := httptest.NewRecorder()
	router.ServeHTTP(wCandResp, reqCandidate)
	assert.Equal(t, http.StatusForbidden, wCandResp.Code)

	// 5. Test RBAC endpoint: Unauthenticated access must be UNAUTHORIZED (401)
	reqUnauth := httptest.NewRequest(http.MethodGet, "/admin/secret", nil)
	wUnauthResp := httptest.NewRecorder()
	router.ServeHTTP(wUnauthResp, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, wUnauthResp.Code)

	// 6. Test RBAC endpoint: Admin access must be GRANTED (200)
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin/secret", nil)
	reqAdmin.AddCookie(adminCookie)
	wAdminResp := httptest.NewRecorder()
	router.ServeHTTP(wAdminResp, reqAdmin)
	assert.Equal(t, http.StatusOK, wAdminResp.Code)
	assert.Contains(t, wAdminResp.Body.String(), "admin_ok")
}

func TestRateLimiter_OnAuthenticationEndpoints(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// Rate limit is set to 5 per minute in testIdentitySetup()
	payload := `{"email":"wrong@example.com","password":"WrongPassword123!"}`

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := jsonReq(http.MethodPost, "/auth/login", payload)
		req.RemoteAddr = "198.51.100.5:12345"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// 6th request from same IP must be BLOCKED by rate limiter (429)
	wBlocked := httptest.NewRecorder()
	reqBlocked := jsonReq(http.MethodPost, "/auth/login", payload)
	reqBlocked.RemoteAddr = "198.51.100.5:12345"
	router.ServeHTTP(wBlocked, reqBlocked)

	assert.Equal(t, http.StatusTooManyRequests, wBlocked.Code)
	assert.Contains(t, wBlocked.Body.String(), "RATE_LIMITED")
}

func TestContentTypeEnforcement_OnAuthEndpoints(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// A. POST /auth/register with text/plain must be rejected (415)
	req1 := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"text@example.com"}`))
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusUnsupportedMediaType, w1.Code)
	assert.Contains(t, w1.Body.String(), "UNSUPPORTED_MEDIA_TYPE")

	// B. POST /auth/login with text/plain must be rejected (415)
	req2 := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"text@example.com"}`))
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnsupportedMediaType, w2.Code)
	assert.Contains(t, w2.Body.String(), "UNSUPPORTED_MEDIA_TYPE")
}

func TestPreAuthCSRF_Discovery_And_Enforcement(t *testing.T) {
	_, _, _, _, router := testIdentitySetup()

	// 1. Discovery: GET /auth/csrf returns token in JSON and cookie
	wDiscovery := httptest.NewRecorder()
	router.ServeHTTP(wDiscovery, httptest.NewRequest(http.MethodGet, "/auth/csrf", nil))
	assert.Equal(t, http.StatusOK, wDiscovery.Code)

	var csrfResp map[string]string
	err := json.NewDecoder(wDiscovery.Body).Decode(&csrfResp)
	require.NoError(t, err)
	token := csrfResp["csrf_token"]
	assert.NotEmpty(t, token)

	var csrfCookie *http.Cookie
	for _, c := range wDiscovery.Result().Cookies() {
		if c.Name == CookieCSRFName {
			csrfCookie = c
		}
	}
	require.NotNil(t, csrfCookie)
	assert.Equal(t, token, csrfCookie.Value)

	// 2. Calling /auth/register with cookie and MISMATCHED header must be rejected (403)
	badReq := jsonReq(http.MethodPost, "/auth/register", `{"email":"preauth_bad@example.com","password":"Password123!","full_name":"Tester"}`)
	badReq.AddCookie(csrfCookie)
	badReq.Header.Set(HeaderXCSRFToken, "tampered-token")
	wBad := httptest.NewRecorder()
	router.ServeHTTP(wBad, badReq)
	assert.Equal(t, http.StatusForbidden, wBad.Code)
	assert.Contains(t, wBad.Body.String(), "INVALID_CSRF_TOKEN")

	// 3. Calling /auth/register with cookie and MATCHING header must succeed (201)
	goodReq := jsonReq(http.MethodPost, "/auth/register", `{"email":"preauth_good@example.com","password":"Password123!","full_name":"Tester"}`)
	goodReq.AddCookie(csrfCookie)
	goodReq.Header.Set(HeaderXCSRFToken, token)
	wGood := httptest.NewRecorder()
	router.ServeHTTP(wGood, goodReq)
	assert.Equal(t, http.StatusCreated, wGood.Code)
}

func TestInactiveUser_CannotAuthenticate_Or_UseExistingSession(t *testing.T) {
	_, _, repo, _, router := testIdentitySetup()

	// 1. Register active user
	regPayload := `{"email":"inactive@example.com","password":"Password123!","full_name":"Inactive User"}`
	wReg := httptest.NewRecorder()
	router.ServeHTTP(wReg, jsonReq(http.MethodPost, "/auth/register", regPayload))
	assert.Equal(t, http.StatusCreated, wReg.Code)

	var sessionCookie *http.Cookie
	for _, c := range wReg.Result().Cookies() {
		if c.Name == "test_session" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)

	// 2. Verify active session works initially
	reqActive := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	reqActive.AddCookie(sessionCookie)
	wActive := httptest.NewRecorder()
	router.ServeHTTP(wActive, reqActive)
	assert.Equal(t, http.StatusOK, wActive.Code)

	// 3. Deactivate user account in repository
	for _, u := range repo.users {
		if u.Email == "inactive@example.com" {
			u.IsActive = false
		}
	}

	// 4. Try authenticating via /auth/login -> rejected (403 ACCOUNT_INACTIVE)
	wLogin := httptest.NewRecorder()
	router.ServeHTTP(wLogin, jsonReq(http.MethodPost, "/auth/login", `{"email":"inactive@example.com","password":"Password123!"}`))
	assert.Equal(t, http.StatusForbidden, wLogin.Code)
	assert.Contains(t, wLogin.Body.String(), "ACCOUNT_INACTIVE")

	// 5. Try using existing session via /auth/me -> rejected (401 UNAUTHORIZED)
	reqInactive := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	reqInactive.AddCookie(sessionCookie)
	wInactive := httptest.NewRecorder()
	router.ServeHTTP(wInactive, reqInactive)
	assert.Equal(t, http.StatusUnauthorized, wInactive.Code)
}

// failingRepo wraps Repository to simulate database failures during multi-step atomic operations.
type failingRepo struct {
	Repository
	failAuditLog bool
}

func (f *failingRepo) CreateAuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{}) (*AuditLog, error) {
	if f.failAuditLog {
		return nil, errors.New("simulated database disk failure on audit_logs")
	}
	return f.Repository.CreateAuditLog(ctx, actorID, action, targetType, targetID, ip, userAgent, metadata)
}

func (f *failingRepo) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	return f.Repository.ExecTx(ctx, func(inner Repository) error {
		txFailing := &failingRepo{
			Repository:   inner,
			failAuditLog: f.failAuditLog,
		}
		return fn(txFailing)
	})
}

func TestTransactionRollback_OnFailure(t *testing.T) {
	innerRepo := newInMemoryRepository()
	failing := &failingRepo{
		Repository:   innerRepo,
		failAuditLog: true, // Audit log will fail
	}

	cfg := &config.Config{
		AppEnv: config.EnvTest,
		Server: config.ServerConfig{Port: 8080},
		Session: config.SessionConfig{
			CookieName:   "test_session",
			CookieExpiry: 24 * time.Hour,
		},
		Auth: config.AuthConfig{
			CSRFSecret: "test-csrf-secret-key-at-least-32-bytes!",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(failing, cfg, logger)
	ctx := context.Background()

	// 1. Attempt candidate registration when audit log fails
	_, _, _, _, err := svc.Register(ctx, "rollback@example.com", "Password123!", "Rollback User", "127.0.0.1", "TestAgent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated database disk failure")

	// Verify candidate was NOT created in repository (transaction was rolled back)
	_, _, err = innerRepo.GetUserByEmail(ctx, "rollback@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound, "user creation must be rolled back on atomic operation failure")
	assert.Empty(t, innerRepo.sessions, "no dangling session should exist after rollback")

	// 2. Attempt administrator creation when audit log fails
	_, err = svc.CreateAdmin(ctx, "admin_rollback@example.com", "AdminPassword123!", "Admin Rollback", "127.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated database disk failure")

	// Verify admin was NOT created in repository
	_, _, err = innerRepo.GetUserByEmail(ctx, "admin_rollback@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound, "admin user creation must be rolled back if audit log fails")
}
