package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/adaptive"
	"github.com/adaptive-dsa/platform/internal/ai"
	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPhase4E2ETestServer(t *testing.T) (*httptest.Server, *config.Config, identity.Service, credits.Service, *uuid.UUID, *http.Cookie, *uuid.UUID, *http.Cookie) {
	t.Helper()

	cfg := &config.Config{
		AppEnv: config.EnvTest,
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 2 * time.Second,
		},
		Database: config.DatabaseConfig{
			URL: "postgres://user:pass@localhost:5432/db",
		},
		Session: config.SessionConfig{
			CookieName:   "session_token",
			CookieExpiry: 24 * time.Hour,
			CookieSecure: false,
		},
		Auth: config.AuthConfig{
			CSRFSecret:       "super-secret-csrf-key-min-32-bytes!",
			RateLimitAuthRPM: 15,
		},
		AI: config.AIConfig{
			GeminiModel:    "gemini-1.5-flash",
			GeminiTimeout:  5 * time.Second,
			GeminiRPM:      15,
			WelcomeCredits: 50,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// In-memory repositories
	idRepo := identity.NewInMemoryRepository()
	idSvc := identity.NewService(idRepo, cfg, logger)

	catRepo := catalog.NewInMemoryRepository()
	catSvc := catalog.NewService(catRepo)

	progRepo := progress.NewInMemoryRepository()
	progSvc := progress.NewService(progRepo)

	credRepo := credits.NewInMemoryRepository()
	credSvc := credits.NewService(credRepo)

	aiRepo := ai.NewInMemoryRepository()
	mockAI := ai.NewMockAIProvider()
	aiSvc := ai.NewService(aiRepo, mockAI, credSvc, catSvc)

	adaptRepo := adaptive.NewInMemoryRepository()
	adaptSvc := adaptive.NewService(adaptRepo, progSvc, catSvc)

	// Create test Candidate user + session
	candidateUser, err := idRepo.CreateUser(ctx, "candidate@example.com", "$argon2id$mockhash", "Candidate Test", identity.RoleCandidate)
	require.NoError(t, err)
	_, err = idRepo.CreateSession(ctx, candidateUser.ID, identity.HashSessionToken("cand-token"), "", "", time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	// Grant 50 welcome credits to candidate
	_, err = credSvc.EnsureAccount(ctx, candidateUser.ID, 50)
	require.NoError(t, err)

	candCookie := &http.Cookie{
		Name:     cfg.Session.CookieName,
		Value:    "cand-token",
		Path:     "/",
		HttpOnly: true,
	}

	// Create test Admin user + session
	adminUser, err := idRepo.CreateUser(ctx, "admin@example.com", "$argon2id$mockhash", "Admin User", identity.RoleAdmin)
	require.NoError(t, err)
	_, err = idRepo.CreateSession(ctx, adminUser.ID, identity.HashSessionToken("admin-token"), "", "", time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	adminCookie := &http.Cookie{
		Name:     cfg.Session.CookieName,
		Value:    "admin-token",
		Path:     "/",
		HttpOnly: true,
	}

	// Create a test question in catalog
	qID := uuid.New()
	_, _ = catRepo.UpsertQuestion(ctx, &catalog.Question{
		ID:          qID,
		Title:       "Two Sum",
		Slug:        "two-sum",
		ExternalURL: "https://leetcode.com/problems/two-sum",
		Difficulty:  catalog.DifficultyEasy,
		IsCore:      true,
	})

	srv := NewServer(cfg, logger, nil, idSvc, catSvc, progSvc, credSvc, aiSvc, adaptSvc, nil)
	ts := httptest.NewServer(srv.Handler())

	return ts, cfg, idSvc, credSvc, &candidateUser.ID, candCookie, &adminUser.ID, adminCookie
}

func TestPhase4_CreditsAndAI_EndToEndAPI(t *testing.T) {
	ts, _, _, credSvc, candID, candCookie, _, adminCookie := setupPhase4E2ETestServer(t)
	defer ts.Close()
	client := ts.Client()

	t.Run("GET /api/v1/credits/balance returns candidate balance", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/credits/balance", nil)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body credits.BalanceResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, int32(50), body.Balance)
	})

	t.Run("POST /api/v1/ai/hints consumes credits and returns hint", func(t *testing.T) {
		payload := map[string]any{
			"level":           1, // Level 1 Nudge: 2 credits
			"candidate_notes": "I tried sorting the array first",
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/hints", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var res ai.HintResult
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, ai.HintLevelNudge, res.Level)
		assert.Equal(t, int32(2), res.CreditsDeducted)
		assert.Equal(t, int32(48), res.RemainingBalance)
		assert.NotEmpty(t, res.HintText)
	})

	t.Run("POST /api/v1/ai/hints returns 402 when credits insufficient", func(t *testing.T) {
		// Deduct 48 credits so balance becomes 0
		_, _ = credSvc.CommitDeduction(context.Background(), *candID, 48, credits.EntryTypeAIUsage, "TEST", nil, "Drain")

		payload := map[string]any{
			"level": 1,
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/hints", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusPaymentRequired, resp.StatusCode)
	})

	t.Run("POST /api/v1/admin/users/{id}/credits rejects non-admin candidate", func(t *testing.T) {
		payload := map[string]any{
			"amount": 100,
			"reason": "Promotional bonus",
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/users/"+candID.String()+"/credits", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie) // candidate cookie

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("POST /api/v1/admin/users/{id}/credits succeeds for admin", func(t *testing.T) {
		payload := map[string]any{
			"amount": 100,
			"reason": "Admin promotional grant",
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/users/"+candID.String()+"/credits", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie) // admin cookie

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Candidate balance should now be 100
		bal, _ := credSvc.GetBalance(context.Background(), *candID)
		assert.Equal(t, int32(100), bal)
	})

	t.Run("GET /api/v1/adaptive/daily returns daily preparation recommendations", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/adaptive/daily", nil)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var plan adaptive.DailyPlan
		err = json.NewDecoder(resp.Body).Decode(&plan)
		require.NoError(t, err)
		assert.NotEmpty(t, plan.Date)
	})

	t.Run("GET /api/v1/adaptive/weak-patterns returns candidate weak patterns", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/adaptive/weak-patterns", nil)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET /api/v1/ai/pricing returns authoritative backend credit pricing", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/ai/pricing", nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body ai.PricingResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		// Authoritative pricing contract assertions
		assert.Equal(t, int32(2), body.Hints["1"], "Level 1 Nudge must cost 2 credits")
		assert.Equal(t, int32(2), body.Hints["2"], "Level 2 Structural must cost 2 credits")
		assert.Equal(t, int32(4), body.Hints["3"], "Level 3 Invariant must cost 4 credits")
		assert.Equal(t, int32(4), body.Hints["4"], "Level 4 Pseudocode must cost 4 credits")
		assert.Equal(t, int32(5), body.ApproachReview, "Approach Review must cost 5 credits")
		assert.Equal(t, int32(3), body.ConceptExplain, "Concept Explain must cost 3 credits")
	})

	t.Run("POST /api/v1/ai/reviews consumes 5 credits and returns review", func(t *testing.T) {
		payload := map[string]any{
			"candidate_code": "def solution(): return True",
			"proposed_time":  "O(1)",
			"proposed_space": "O(1)",
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/reviews", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var res ai.ReviewResult
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, int32(5), res.CreditsDeducted)
		assert.NotEmpty(t, res.ReviewText)
	})

	t.Run("POST /api/v1/ai/explain consumes 3 credits and returns explanation", func(t *testing.T) {
		payload := map[string]any{
			"topic_name":      "Trees",
			"pattern_name":    "DFS",
			"candidate_query": "What is preorder traversal?",
		}
		jsonBytes, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ai/explain", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var res ai.ExplainResult
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, int32(3), res.CreditsDeducted)
		assert.NotEmpty(t, res.Explanation)
	})
}
