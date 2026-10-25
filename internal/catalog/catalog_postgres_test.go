package catalog_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestPostgresPool(t *testing.T) (*database.DB, catalog.Repository) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL integration test: DATABASE_URL not set")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    3 * time.Second,
	}

	ctx := context.Background()
	db, err := database.New(ctx, cfg, logger)
	require.NoError(t, err)

	repo := catalog.NewPostgresRepository(db.Pool())
	return db, repo
}

func TestCatalog_Postgres_TopicAndPatternHierarchy(t *testing.T) {
	db, repo := getTestPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create Topic
	uniqueSuffix := uuid.New().String()[:8]
	topicName := "Test Topic " + uniqueSuffix
	topicSlug := "test-topic-" + uniqueSuffix
	topicDesc := "A test topic description"

	createdTopic, err := repo.CreateTopic(ctx, &catalog.Topic{
		Name:         topicName,
		Slug:         topicSlug,
		Description:  &topicDesc,
		DisplayOrder: 99,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, createdTopic.ID)
	assert.Equal(t, topicName, createdTopic.Name)

	// 2. Query Topic by slug and name
	bySlug, err := repo.GetTopicBySlug(ctx, topicSlug)
	require.NoError(t, err)
	assert.Equal(t, createdTopic.ID, bySlug.ID)

	byName, err := repo.GetTopicByName(ctx, topicName)
	require.NoError(t, err)
	assert.Equal(t, createdTopic.ID, byName.ID)

	// 3. Create Parent Pattern
	patternName := "Test Pattern " + uniqueSuffix
	patternSlug := "test-pattern-" + uniqueSuffix
	parentPattern, err := repo.CreatePattern(ctx, &catalog.Pattern{
		TopicID:      &createdTopic.ID,
		Name:         patternName,
		Slug:         patternSlug,
		DisplayOrder: 1,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, parentPattern.ID)

	// 4. Create Child Subpattern
	subName := "Test Subpattern " + uniqueSuffix
	subSlug := "test-subpattern-" + uniqueSuffix
	childPattern, err := repo.CreatePattern(ctx, &catalog.Pattern{
		TopicID:      &createdTopic.ID,
		ParentID:     &parentPattern.ID,
		Name:         subName,
		Slug:         subSlug,
		DisplayOrder: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, parentPattern.ID, *childPattern.ParentID)

	// 5. Query Patterns by Topic
	patterns, err := repo.ListPatternsByTopic(ctx, createdTopic.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(patterns), 2)
}

func TestCatalog_Postgres_QuestionUpsertAndQuery(t *testing.T) {
	db, repo := getTestPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	uniqueSuffix := uuid.New().String()[:8]
	qTitle := "Test Question " + uniqueSuffix
	qSlug := "test-question-" + uniqueSuffix
	qURL := "https://leetcode.com/problems/" + qSlug + "/"
	edSummary := "Test summary"

	// 1. Initial Upsert
	q1, err := repo.UpsertQuestion(ctx, &catalog.Question{
		Title:               qTitle,
		Slug:                qSlug,
		ExternalURL:         qURL,
		Difficulty:          catalog.DifficultyMedium,
		IsCore:              true,
		EditorialSummary:    &edSummary,
		SampleTimeLimitMins: 35,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, q1.ID)
	assert.Equal(t, catalog.DifficultyMedium, q1.Difficulty)

	// 2. Re-Upsert with updated difficulty (Idempotent update by URL)
	q2, err := repo.UpsertQuestion(ctx, &catalog.Question{
		Title:               qTitle,
		Slug:                qSlug,
		ExternalURL:         qURL,
		Difficulty:          catalog.DifficultyHard,
		IsCore:              true,
		EditorialSummary:    &edSummary,
		SampleTimeLimitMins: 45,
	})
	require.NoError(t, err)
	assert.Equal(t, q1.ID, q2.ID, "canonical UUID must be preserved across updates")
	assert.Equal(t, catalog.DifficultyHard, q2.Difficulty)
	assert.Equal(t, int32(45), q2.SampleTimeLimitMins)

	// 3. Retrieve by ID, Slug, and URL
	byID, err := repo.GetQuestionByID(ctx, q1.ID)
	require.NoError(t, err)
	assert.Equal(t, q1.ID, byID.ID)

	bySlug, err := repo.GetQuestionBySlug(ctx, qSlug)
	require.NoError(t, err)
	assert.Equal(t, q1.ID, bySlug.ID)

	byURL, err := repo.GetQuestionByExternalURL(ctx, qURL)
	require.NoError(t, err)
	assert.Equal(t, q1.ID, byURL.ID)
}

func TestCatalog_Postgres_RoadmapsAndOrderedItems(t *testing.T) {
	db, repo := getTestPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	uniqueSuffix := uuid.New().String()[:8]
	rTitle := "Test Roadmap " + uniqueSuffix
	rSlug := "test-roadmap-" + uniqueSuffix
	rDesc := "Test roadmap description"

	// 1. Create Roadmap
	createdRoadmap, err := repo.CreateRoadmap(ctx, &catalog.Roadmap{
		Title:            rTitle,
		Slug:             rSlug,
		Description:      &rDesc,
		TargetDays:       45,
		IsSystemTemplate: true,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, createdRoadmap.ID)

	// 2. Create 2 questions to link
	q1, err := repo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Roadmap Q1 " + uniqueSuffix,
		Slug:        "roadmap-q1-" + uniqueSuffix,
		ExternalURL: "https://leetcode.com/problems/roadmap-q1-" + uniqueSuffix + "/",
		Difficulty:  catalog.DifficultyEasy,
	})
	require.NoError(t, err)

	q2, err := repo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Roadmap Q2 " + uniqueSuffix,
		Slug:        "roadmap-q2-" + uniqueSuffix,
		ExternalURL: "https://leetcode.com/problems/roadmap-q2-" + uniqueSuffix + "/",
		Difficulty:  catalog.DifficultyMedium,
	})
	require.NoError(t, err)

	// 3. Add Roadmap Items with explicit sort order
	err = repo.AddRoadmapItem(ctx, createdRoadmap.ID, q1.ID, 1, 1, true)
	require.NoError(t, err)

	err = repo.AddRoadmapItem(ctx, createdRoadmap.ID, q2.ID, 1, 2, true)
	require.NoError(t, err)

	// 4. List Roadmap Items and verify ordered sequence
	items, err := repo.ListRoadmapItems(ctx, createdRoadmap.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, q1.ID, items[0].QuestionID)
	assert.Equal(t, int32(1), items[0].OrderIndex)
	assert.Equal(t, q2.ID, items[1].QuestionID)
	assert.Equal(t, int32(2), items[1].OrderIndex)
}

func TestCatalog_Postgres_TransactionRollback(t *testing.T) {
	db, repo := getTestPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	uniqueSuffix := uuid.New().String()[:8]
	topicSlug := "tx-rollback-topic-" + uniqueSuffix

	// Execute transaction that fails intentionally at step 2
	err := repo.ExecTx(ctx, func(txRepo catalog.Repository) error {
		_, err := txRepo.CreateTopic(ctx, &catalog.Topic{
			Name: "Tx Topic " + uniqueSuffix,
			Slug: topicSlug,
		})
		if err != nil {
			return err
		}

		// Force failure to trigger rollback
		return errors.New("simulated fatal failure inside transaction")
	})
	require.Error(t, err)

	// Verify topic was NOT persisted in PostgreSQL
	_, getErr := repo.GetTopicBySlug(ctx, topicSlug)
	assert.ErrorIs(t, getErr, catalog.ErrTopicNotFound, "rolled-back topic must not exist in database")
}
