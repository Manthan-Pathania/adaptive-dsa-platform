package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeminiProvider connects to the Google Gemini Generative Language REST API.
type GeminiProvider struct {
	apiKey     string
	model      string
	timeout    time.Duration
	httpClient *http.Client
}

func NewGeminiProvider(apiKey, model string, timeout time.Duration) *GeminiProvider {
	if model == "" {
		model = "gemini-3.8-flash"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &GeminiProvider{
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ModelName returns the configured model identifier.
func (g *GeminiProvider) ModelName() string {
	return g.model
}

// Request and response payloads matching Google Gemini REST API v1beta
type geminiContentPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string              `json:"role,omitempty"`
	Parts []geminiContentPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int32 `json:"promptTokenCount"`
	CandidatesTokenCount int32 `json:"candidatesTokenCount"`
	TotalTokenCount      int32 `json:"totalTokenCount"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
	Error         *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (g *GeminiProvider) callGemini(ctx context.Context, systemInstruction, userPrompt string) (*AIResponse, error) {
	if g.apiKey == "" {
		return nil, fmt.Errorf("gemini: api key is not configured")
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiContentPart{
					{Text: userPrompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     0.2, // Low temperature for factual, deterministic tutor guidance
			MaxOutputTokens: 800,
		},
	}

	if systemInstruction != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiContentPart{
				{Text: systemInstruction},
			},
		}
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		g.model,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrQuotaExceeded
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d (%s)", ErrProviderUnavailable, resp.StatusCode, string(bodyBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("gemini: unmarshal response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini api error %d: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: received empty response candidate from provider")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	return &AIResponse{
		Content:          content,
		PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
		CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		ModelUsed:        g.model,
	}, nil
}

func (g *GeminiProvider) GenerateHint(ctx context.Context, req GenerateHintRequest) (*AIResponse, error) {
	sys, user := BuildHintPrompt(req)
	return g.callGemini(ctx, sys, user)
}

func (g *GeminiProvider) ReviewApproach(ctx context.Context, req ReviewApproachRequest) (*AIResponse, error) {
	sys, user := BuildReviewPrompt(req)
	return g.callGemini(ctx, sys, user)
}

func (g *GeminiProvider) ExplainConcept(ctx context.Context, req ExplainConceptRequest) (*AIResponse, error) {
	sys, user := BuildExplainPrompt(req)
	return g.callGemini(ctx, sys, user)
}

func (g *GeminiProvider) GenerateCustomRoadmap(ctx context.Context, req GenerateRoadmapRequest) (*GenerateRoadmapResponse, error) {
	sysPrompt := "You are a Principal Software Engineer and elite FAANG DSA Interview Coach. Based on the candidate's target companies, timeline, and experience, create a structured 3-stage DSA preparation roadmap. You MUST select question IDs ONLY from the provided master question catalog. Output strict JSON with format: {\"title\": \"...\", \"description\": \"...\", \"target_days\": 60, \"stages\": [{\"stage_number\": 1, \"title\": \"...\", \"question_ids\": [\"uuid...\"]}]}."

	var catalogSb strings.Builder
	catalogSb.WriteString(fmt.Sprintf("Candidate Targets: %v\nTimeline Days: %d\nDaily Hours: %.1f\nExperience: %s\nFamiliar Patterns: %v\nFocus: %s\n\nAvailable Master Question Catalog (ID, Title, Pattern, Difficulty):\n",
		req.TargetCompanies, req.TimelineDays, req.DailyHours, req.ExperienceLevel, req.FamiliarPatterns, req.FocusNotes))

	for i, q := range req.Questions {
		if i >= 120 {
			break
		}
		catalogSb.WriteString(fmt.Sprintf("- %s: %s (%s, %s)\n", q.ID.String(), q.Title, q.Pattern, q.Difficulty))
	}

	resp, err := g.callGemini(ctx, sysPrompt, catalogSb.String())
	if err != nil {
		fallback := NewMockAIProvider()
		return fallback.GenerateCustomRoadmap(ctx, req)
	}

	cleanJSON := strings.TrimSpace(resp.Content)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var result GenerateRoadmapResponse
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		fallback := NewMockAIProvider()
		return fallback.GenerateCustomRoadmap(ctx, req)
	}

	if len(result.Stages) == 0 {
		fallback := NewMockAIProvider()
		return fallback.GenerateCustomRoadmap(ctx, req)
	}

	return &result, nil
}
