package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/catalog/importer"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func createTestExcelBuffer(t *testing.T, headers []string, data [][]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Master Plan"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1")

	for colIdx, h := range headers {
		cell, err := excelize.CoordinatesToCellName(colIdx+1, 1)
		require.NoError(t, err)
		err = f.SetCellValue(sheet, cell, h)
		require.NoError(t, err)
	}

	for rowIdx, row := range data {
		for colIdx, val := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			require.NoError(t, err)
			err = f.SetCellValue(sheet, cell, val)
			require.NoError(t, err)
		}
	}

	buf := new(bytes.Buffer)
	err := f.Write(buf)
	require.NoError(t, err)
	return buf
}

func TestCatalogAdminAPI_CompleteLifecycle(t *testing.T) {
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

	idRepo := identity.NewInMemoryRepository()
	idSvc := identity.NewService(idRepo, cfg, logger)

	catRepo := catalog.NewInMemoryRepository()
	catImp := importer.New(catRepo)
	catSvc := catalog.NewService(catRepo, catImp)

	ctx := context.Background()

	// 1. Create candidate and admin users
	candUser, err := idRepo.CreateUser(ctx, "candidate@catalog.com", "$argon2id$mockhash", "Candidate User", identity.RoleCandidate)
	require.NoError(t, err)
	adminUser, err := idRepo.CreateUser(ctx, "admin@catalog.com", "$argon2id$mockhash", "Admin User", identity.RoleAdmin)
	require.NoError(t, err)

	candHash := identity.HashSessionToken("cand-token")
	adminHash := identity.HashSessionToken("admin-token")
	_, err = idRepo.CreateSession(ctx, candUser.ID, candHash, "127.0.0.1", "test-agent", time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	_, err = idRepo.CreateSession(ctx, adminUser.ID, adminHash, "127.0.0.1", "test-agent", time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	candCookie := &http.Cookie{Name: cfg.Session.CookieName, Value: "cand-token", Path: "/"}
	adminCookie := &http.Cookie{Name: cfg.Session.CookieName, Value: "admin-token", Path: "/"}

	srv := NewServer(cfg, logger, nil, idSvc, catSvc, nil, nil, nil, nil, nil)
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	// 2. Test RBAC: Candidate cannot access GET /api/v1/admin/questions
	t.Run("GET /api/v1/admin/questions rejects candidate with 403", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/questions", nil)
		req.AddCookie(candCookie)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// 3. Test RBAC: Admin can access GET /api/v1/admin/questions
	t.Run("GET /api/v1/admin/questions succeeds for admin", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/questions", nil)
		req.AddCookie(adminCookie)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res catalog.PaginatedQuestionsResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, int64(0), res.Total)
	})

	// 4. Test Manual Add Question: POST /api/v1/admin/questions
	var createdID uuid.UUID
	t.Run("POST /api/v1/admin/questions creates a question", func(t *testing.T) {
		payload := map[string]interface{}{
			"title":                  "Binary Search",
			"difficulty":             "EASY",
			"is_core":                true,
			"sample_time_limit_mins": 25,
			"leetcode_id":            "704",
		}
		jsonBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/questions", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var q catalog.Question
		err = json.NewDecoder(resp.Body).Decode(&q)
		require.NoError(t, err)
		assert.Equal(t, "Binary Search", q.Title)
		assert.True(t, q.IsActive)
		assert.Equal(t, "https://leetcode.com/problems/binary-search", q.ExternalURL)
		require.NotNil(t, q.LeetCodeID)
		assert.Equal(t, "704", *q.LeetCodeID)
		createdID = q.ID
	})

	// 5. Test Edit Question: PUT /api/v1/admin/questions/{id}
	t.Run("PUT /api/v1/admin/questions/{id} updates question fields", func(t *testing.T) {
		payload := map[string]interface{}{
			"title":                  "Binary Search (Classic Algorithm)",
			"difficulty":             "EASY",
			"is_core":                false,
			"sample_time_limit_mins": 20,
			"leetcode_id":            "704",
		}
		jsonBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/admin/questions/"+createdID.String(), bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var q catalog.Question
		err = json.NewDecoder(resp.Body).Decode(&q)
		require.NoError(t, err)
		assert.Equal(t, "Binary Search (Classic Algorithm)", q.Title)
		assert.False(t, q.IsCore)
	})

	// 6. Test Archive Question: PATCH /api/v1/admin/questions/{id}/archive
	t.Run("PATCH /api/v1/admin/questions/{id}/archive toggles active status", func(t *testing.T) {
		// Archive
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/admin/questions/"+createdID.String()+"/archive", nil)
		req.AddCookie(adminCookie)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var q catalog.Question
		err = json.NewDecoder(resp.Body).Decode(&q)
		require.NoError(t, err)
		assert.False(t, q.IsActive)

		// Candidate catalog hides archived question
		candReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/questions", nil)
		candResp, err := client.Do(candReq)
		require.NoError(t, err)
		defer candResp.Body.Close()
		var candRes catalog.PaginatedQuestionsResponse
		_ = json.NewDecoder(candResp.Body).Decode(&candRes)
		assert.Equal(t, int64(0), candRes.Total)

		// Restore
		restoreReq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/admin/questions/"+createdID.String()+"/archive", nil)
		restoreReq.AddCookie(adminCookie)
		restoreResp, err := client.Do(restoreReq)
		require.NoError(t, err)
		defer restoreResp.Body.Close()
		assert.Equal(t, http.StatusOK, restoreResp.StatusCode)
		var restoredQ catalog.Question
		_ = json.NewDecoder(restoreResp.Body).Decode(&restoredQ)
		assert.True(t, restoredQ.IsActive)

		// Candidate catalog now sees it
		candReq2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/questions", nil)
		candResp2, err := client.Do(candReq2)
		require.NoError(t, err)
		defer candResp2.Body.Close()
		var candRes2 catalog.PaginatedQuestionsResponse
		_ = json.NewDecoder(candResp2.Body).Decode(&candRes2)
		assert.Equal(t, int64(1), candRes2.Total)
	})

	// 7. Test Bulk Excel Upload Preview & Validation
	headers := []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem", "LeetCode ID", "Progress", "Notes"}
	sampleData := [][]string{
		{"1", "Arrays & Hashing", "Frequency counting", "Easy", "Contains Duplicate", "217", "Done", "Use set"},
		{"2", "Arrays & Hashing", "Two Pointers", "Easy", "Move Zeroes", "283", "Attempted", "Swap forward"},
		{"3", "Two Pointers & Sliding Window", "Pointers", "Easy", "Move Zeroes", "283", "Done", "Cross-listed problem"},
	}
	excelBuf := createTestExcelBuffer(t, headers, sampleData)

	t.Run("POST /api/v1/admin/questions/import/preview returns validation scorecard", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "FAANG_DSA_Master_Plan.xlsx")
		require.NoError(t, err)
		_, err = io.Copy(part, bytes.NewReader(excelBuf.Bytes()))
		require.NoError(t, err)
		writer.Close()

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/questions/import/preview", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.AddCookie(adminCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var preview catalog.ImportPreviewResult
		err = json.NewDecoder(resp.Body).Decode(&preview)
		require.NoError(t, err)
		assert.Equal(t, "Master Plan", preview.DetectedSheet)
		assert.Equal(t, 3, preview.TotalRowsRead)
		assert.Equal(t, 3, preview.ValidRows)
		assert.Equal(t, 0, preview.InvalidRows)
		assert.Equal(t, 2, preview.UniqueQuestions)
		assert.Equal(t, 1, preview.CrossListedRows)
		assert.Equal(t, 2, preview.PlannedCreates)
	})

	// 8. Test Bulk Excel Upload Commit & Import History
	t.Run("POST /api/v1/admin/questions/import/commit commits data and logs history", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "FAANG_DSA_Master_Plan.xlsx")
		require.NoError(t, err)
		_, err = io.Copy(part, bytes.NewReader(excelBuf.Bytes()))
		require.NoError(t, err)
		writer.Close()

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/questions/import/commit", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.AddCookie(adminCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res catalog.ImportResult
		err = json.NewDecoder(resp.Body).Decode(&res)
		require.NoError(t, err)
		assert.Equal(t, 3, res.TotalRowsRead)
		assert.Equal(t, 3, res.ValidRows)
		assert.Equal(t, 2, res.CreatedQuestions)
		assert.Equal(t, 1, res.CrossListedRows)

		// Verify questions now appear in candidate catalog
		candReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/questions?search=Duplicate", nil)
		candResp, err := client.Do(candReq)
		require.NoError(t, err)
		defer candResp.Body.Close()
		var candRes catalog.PaginatedQuestionsResponse
		_ = json.NewDecoder(candResp.Body).Decode(&candRes)
		assert.Equal(t, int64(1), candRes.Total)
		assert.Equal(t, "Contains Duplicate", candRes.Questions[0].Title)

		// Verify import history: GET /api/v1/admin/questions/import/history
		histReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/questions/import/history", nil)
		histReq.AddCookie(adminCookie)
		histResp, err := client.Do(histReq)
		require.NoError(t, err)
		defer histResp.Body.Close()
		assert.Equal(t, http.StatusOK, histResp.StatusCode)

		var histData struct {
			History []catalog.ImportHistory `json:"history"`
		}
		err = json.NewDecoder(histResp.Body).Decode(&histData)
		require.NoError(t, err)
		require.NotEmpty(t, histData.History)
		assert.Equal(t, "FAANG_DSA_Master_Plan.xlsx", histData.History[0].Filename)
		assert.Equal(t, int32(3), histData.History[0].TotalRows)
		assert.Equal(t, int32(2), histData.History[0].CreatedQuestions)
		assert.Equal(t, "COMPLETED", histData.History[0].Status)
	})
}
