package ai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockAIProvider is a configurable, thread-safe AIProvider for unit testing and offline development.
type MockAIProvider struct {
	mu             sync.Mutex
	ForcedError    error
	Delay          time.Duration
	CallCount      int
	LastHintReq    *GenerateHintRequest
	LastReviewReq  *ReviewApproachRequest
	LastExplainReq *ExplainConceptRequest
}

func NewMockAIProvider() *MockAIProvider {
	return &MockAIProvider{}
}

func (m *MockAIProvider) SetForcedError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ForcedError = err
}

func (m *MockAIProvider) SetDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Delay = d
}

func (m *MockAIProvider) GenerateHint(ctx context.Context, req GenerateHintRequest) (*AIResponse, error) {
	m.mu.Lock()
	m.CallCount++
	m.LastHintReq = &req
	err := m.ForcedError
	delay := m.Delay
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	var content string
	switch req.Level {
	case HintLevelNudge:
		content = fmt.Sprintf("Consider whether the input possesses monotonicity or if sorting helps eliminate duplicate comparisons for '%s'.", req.QuestionTitle)
	case HintLevelStructural:
		content = fmt.Sprintf("A '%s' pattern works well here. Maintain two boundary indices to avoid quadratic re-scanning.", req.PatternName)
	case HintLevelInvariant:
		content = "Maintain the invariant that all elements to the left of the pointer satisfy the search constraint. Increment when condition holds."
	case HintLevelPseudocode:
		content = "Pseudocode:\n1. Initialize left=0, right=n-1\n2. While left < right: compute mid\n3. Adjust pointers based on predicate\n4. Handle boundary when target is missing."
	default:
		content = "Standard algorithmic guidance."
	}

	return &AIResponse{
		Content:          content,
		PromptTokens:     250,
		CompletionTokens: 85,
		ModelUsed:        "mock-gemini-flash",
	}, nil
}

func (m *MockAIProvider) ReviewApproach(ctx context.Context, req ReviewApproachRequest) (*AIResponse, error) {
	m.mu.Lock()
	m.CallCount++
	m.LastReviewReq = &req
	err := m.ForcedError
	delay := m.Delay
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	return &AIResponse{
		Content:          fmt.Sprintf("Your proposed approach for '%s' is technically sound. Time complexity: %s, Space complexity: %s. Watch out for edge cases with empty or 1-element inputs.", req.QuestionTitle, req.ProposedTime, req.ProposedSpace),
		PromptTokens:     350,
		CompletionTokens: 120,
		ModelUsed:        "mock-gemini-flash",
	}, nil
}

func (m *MockAIProvider) ExplainConcept(ctx context.Context, req ExplainConceptRequest) (*AIResponse, error) {
	m.mu.Lock()
	m.CallCount++
	m.LastExplainReq = &req
	err := m.ForcedError
	delay := m.Delay
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	return &AIResponse{
		Content:          fmt.Sprintf("The concept '%s' under topic '%s' is essential for interview problem solving. It reduces time complexity from O(N^2) to O(N) by eliminating redundant subproblems.", req.PatternName, req.TopicName),
		PromptTokens:     200,
		CompletionTokens: 90,
		ModelUsed:        "mock-gemini-flash",
	}, nil
}

func (m *MockAIProvider) GenerateCustomRoadmap(ctx context.Context, req GenerateRoadmapRequest) (*GenerateRoadmapResponse, error) {
	m.mu.Lock()
	m.CallCount++
	err := m.ForcedError
	delay := m.Delay
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if err != nil {
		return nil, err
	}

	title := "Custom FAANG Preparation Roadmap"
	if len(req.TargetCompanies) > 0 {
		title = fmt.Sprintf("Targeted %s Interview Roadmap", req.TargetCompanies[0])
		if len(req.TargetCompanies) > 1 {
			title = fmt.Sprintf("%s & %s Interview Roadmap", req.TargetCompanies[0], req.TargetCompanies[1])
		}
	}

	targetDays := req.TimelineDays
	if targetDays <= 0 {
		targetDays = 60
	}

	stage1 := GeneratedRoadmapStage{
		StageNumber: 1,
		Title:       "Stage 1: Core Patterns & Prerequisites",
	}
	stage2 := GeneratedRoadmapStage{
		StageNumber: 2,
		Title:       "Stage 2: Targeted Strengthening & Variations",
	}
	stage3 := GeneratedRoadmapStage{
		StageNumber: 3,
		Title:       "Stage 3: Advanced Transfer & Speed Consolidation",
	}

	for i, q := range req.Questions {
		if i >= 45 {
			break
		}
		if q.Difficulty == "EASY" || i%3 == 0 {
			stage1.QuestionIDs = append(stage1.QuestionIDs, q.ID)
		} else if q.Difficulty == "MEDIUM" || i%3 == 1 {
			stage2.QuestionIDs = append(stage2.QuestionIDs, q.ID)
		} else {
			stage3.QuestionIDs = append(stage3.QuestionIDs, q.ID)
		}
	}

	return &GenerateRoadmapResponse{
		Title:       title,
		Description: fmt.Sprintf("A tailored %d-day DSA interview preparation plan prioritizing core algorithmic patterns and your selected target companies.", targetDays),
		TargetDays:  targetDays,
		Stages:      []GeneratedRoadmapStage{stage1, stage2, stage3},
	}, nil
}
