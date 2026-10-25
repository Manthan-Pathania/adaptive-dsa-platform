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

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogAndProgressEndpoints(t *testing.T) {
	cfg := &config.Config{
		AppEnv: "testing",
		Server: config.ServerConfig{
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 2 * time.Second,
		},
		Session: config.SessionConfig{
			CookieName: "session_token",
		},
		Auth: config.AuthConfig{
			CSRFSecret: "test-secret-min-32-chars-long-abc!",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Repositories & Services
	catRepo := catalog.NewInMemoryRepository()
	catSvc := catalog.NewService(catRepo)

	progRepo := progress.NewInMemoryRepository()
	progSvc := progress.NewService(progRepo)

	idRepo := identity.NewInMemoryRepository()
	idSvc := identity.NewService(idRepo, cfg, logger)

	// Seed test data
	ctx := context.Background()
	user, err := idRepo.CreateUser(ctx, "candidate@example.com", "$argon2id$mockhash", "Candidate User", identity.RoleCandidate)
	require.NoError(t, err)

	q1, err := catRepo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Two Sum",
		Slug:        "two-sum",
		ExternalURL: "https://leetcode.com/problems/two-sum",
		Difficulty:  catalog.DifficultyEasy,
		IsCore:      true,
	})
	require.NoError(t, err)

	rd, err := catRepo.CreateRoadmap(ctx, &catalog.Roadmap{
		Title:            "NeetCode 150",
		Slug:             "neetcode-150",
		IsSystemTemplate: true,
	})
	require.NoError(t, err)
	err = catRepo.AddRoadmapItem(ctx, rd.ID, q1.ID, 1, 1, true)
	require.NoError(t, err)

	srv := NewServer(cfg, logger, nil, idSvc, catSvc, progSvc)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	t.Run("GET /api/v1/questions lists catalog questions", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/questions")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var body catalog.PaginatedQuestionsResponse
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, int64(1), body.Total)
		assert.Equal(t, "Two Sum", body.Questions[0].Title)
	})

	t.Run("GET /api/v1/questions/{id} returns question detail", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/questions/two-sum")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fetched catalog.Question
		err = json.NewDecoder(resp.Body).Decode(&fetched)
		require.NoError(t, err)
		assert.Equal(t, "Two Sum", fetched.Title)
		assert.Equal(t, q1.ID, fetched.ID)
	})

	t.Run("GET /api/v1/roadmaps returns curated roadmaps", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/roadmaps")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string][]catalog.Roadmap
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.NotEmpty(t, body["roadmaps"])
		assert.Equal(t, "NeetCode 150", body["roadmaps"][0].Title)
	})

	t.Run("GET /api/v1/roadmaps/{id} returns roadmap with ordered items", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/roadmaps/neetcode-150")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var fetched catalog.Roadmap
		err = json.NewDecoder(resp.Body).Decode(&fetched)
		require.NoError(t, err)
		assert.Equal(t, "NeetCode 150", fetched.Title)
		require.Equal(t, 1, len(fetched.Items))
		assert.Equal(t, "Two Sum", fetched.Items[0].QuestionTitle)
	})

	t.Run("POST /api/v1/questions/{id}/attempts requires authentication", func(t *testing.T) {
		payload := progress.RecordAttemptPayload{
			Outcome:       string(progress.OutcomeSolvedIndependently),
			TimeSpentSecs: 600,
			QualityRating: 5,
		}
		data, _ := json.Marshal(payload)

		resp, err := client.Post(ts.URL+"/api/v1/questions/"+q1.ID.String()+"/attempts", "application/json", bytes.NewReader(data))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Authenticated attempt recording updates progress and SM-2", func(t *testing.T) {
		// Log in candidate to get valid session cookie
		session, err := idRepo.CreateSession(ctx, user.ID, identity.HashSessionToken("test-session-token"), "", "", time.Now().Add(24*time.Hour))
		require.NoError(t, err)
		require.NotNil(t, session)

		payload := progress.RecordAttemptPayload{
			Outcome:               string(progress.OutcomeSolvedIndependently),
			TimeSpentSecs:         750,
			DifficultyExperienced: string(progress.PerceivedEasy),
			QualityRating:         5,
		}
		data, _ := json.Marshal(payload)

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/questions/"+q1.ID.String()+"/attempts", bytes.NewReader(data))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-session-token")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var recResp progress.RecordAttemptResponse
		err = json.NewDecoder(resp.Body).Decode(&recResp)
		require.NoError(t, err)
		assert.Equal(t, progress.OutcomeSolvedIndependently, recResp.Attempt.Outcome)
		assert.Equal(t, progress.StatusSolved, recResp.Progress.Status)
		assert.Equal(t, int32(1), recResp.Progress.SolveCount)
		assert.Equal(t, int32(1), recResp.Revision.RepetitionCount)

		// Verify GET /api/v1/progress returns updated stats
		progReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/progress", nil)
		progReq.Header.Set("Authorization", "Bearer test-session-token")

		progResp, err := client.Do(progReq)
		require.NoError(t, err)
		defer progResp.Body.Close()

		assert.Equal(t, http.StatusOK, progResp.StatusCode)
		var summary progress.UserProgressSummary
		err = json.NewDecoder(progResp.Body).Decode(&summary)
		require.NoError(t, err)
		assert.Equal(t, int64(1), summary.Stats.TotalSolved)
		assert.Equal(t, 1, len(summary.RecentAttempts))
	})
}
