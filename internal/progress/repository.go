package progress

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides persistence operations for attempts, progress, and spaced repetition.
type Repository interface {
	CreateQuestionAttempt(ctx context.Context, attempt *QuestionAttempt) (*QuestionAttempt, error)
	ListQuestionAttempts(ctx context.Context, userID, questionID uuid.UUID) ([]QuestionAttempt, error)
	ListUserRecentAttempts(ctx context.Context, userID uuid.UUID, limit int32) ([]QuestionAttempt, error)

	GetUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) (*UserQuestionProgress, error)
	ListUserQuestionProgress(ctx context.Context, userID uuid.UUID) ([]UserQuestionProgress, error)
	UpsertUserQuestionProgress(ctx context.Context, p *UserQuestionProgress) (*UserQuestionProgress, error)
	DeleteUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) error
	GetUserProgressStats(ctx context.Context, userID uuid.UUID) (*UserProgressStats, error)

	GetRevisionSchedule(ctx context.Context, userID, questionID uuid.UUID) (*RevisionSchedule, error)
	UpsertRevisionSchedule(ctx context.Context, s *RevisionSchedule) (*RevisionSchedule, error)
	ListDueRevisions(ctx context.Context, userID uuid.UUID, asOf time.Time, limit, offset int32) ([]RevisionSchedule, int64, error)
	ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExportData, error)

	ExecTx(ctx context.Context, fn func(repo Repository) error) error
}

type pgxRepository struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
	q       generated.Querier
}

// NewPostgresRepository creates a pgx-backed repository.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	queries := generated.New(pool)
	return &pgxRepository{
		pool:    pool,
		queries: queries,
		q:       queries,
	}
}

func toUUID(p pgtype.UUID) uuid.UUID {
	return uuid.UUID(p.Bytes)
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func (r *pgxRepository) CreateQuestionAttempt(ctx context.Context, a *QuestionAttempt) (*QuestionAttempt, error) {
	var rid pgtype.UUID
	if a.RoadmapID != nil {
		rid = toPgUUID(*a.RoadmapID)
	}
	var approach pgtype.Text
	if a.ApproachSummary != nil {
		approach = pgtype.Text{String: *a.ApproachSummary, Valid: true}
	}
	var code pgtype.Text
	if a.CodeSnippet != nil {
		code = pgtype.Text{String: *a.CodeSnippet, Valid: true}
	}

	diffExp := generated.PerceivedDifficultyJUSTRIGHT
	if a.DifficultyExperienced != "" {
		diffExp = generated.PerceivedDifficulty(a.DifficultyExperienced)
	}

	mistakeTags := a.MistakeTags
	if mistakeTags == nil {
		mistakeTags = []string{}
	}

	res, err := r.q.CreateQuestionAttempt(ctx, generated.CreateQuestionAttemptParams{
		UserID:                toPgUUID(a.UserID),
		QuestionID:            toPgUUID(a.QuestionID),
		RoadmapID:             rid,
		Outcome:               generated.AttemptOutcome(a.Outcome),
		TimeSpentSecs:         a.TimeSpentSecs,
		DifficultyExperienced: diffExp,
		QualityRating:         a.QualityRating,
		MistakeTags:           mistakeTags,
		ApproachSummary:       approach,
		CodeSnippet:           code,
	})
	if err != nil {
		return nil, fmt.Errorf("create attempt: %w", err)
	}

	var resRoadmapID *uuid.UUID
	if res.RoadmapID.Valid {
		u := uuid.UUID(res.RoadmapID.Bytes)
		resRoadmapID = &u
	}
	var resApproach *string
	if res.ApproachSummary.Valid {
		resApproach = &res.ApproachSummary.String
	}
	var resCode *string
	if res.CodeSnippet.Valid {
		resCode = &res.CodeSnippet.String
	}

	return &QuestionAttempt{
		ID:                    toUUID(res.ID),
		UserID:                toUUID(res.UserID),
		QuestionID:            toUUID(res.QuestionID),
		RoadmapID:             resRoadmapID,
		Outcome:               AttemptOutcome(res.Outcome),
		TimeSpentSecs:         res.TimeSpentSecs,
		DifficultyExperienced: PerceivedDifficulty(res.DifficultyExperienced),
		QualityRating:         res.QualityRating,
		MistakeTags:           res.MistakeTags,
		ApproachSummary:       resApproach,
		CodeSnippet:           resCode,
		CreatedAt:             res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListQuestionAttempts(ctx context.Context, userID, questionID uuid.UUID) ([]QuestionAttempt, error) {
	rows, err := r.q.ListQuestionAttempts(ctx, generated.ListQuestionAttemptsParams{
		UserID:     toPgUUID(userID),
		QuestionID: toPgUUID(questionID),
	})
	if err != nil {
		return nil, err
	}
	attempts := make([]QuestionAttempt, len(rows))
	for i, res := range rows {
		var resRoadmapID *uuid.UUID
		if res.RoadmapID.Valid {
			u := uuid.UUID(res.RoadmapID.Bytes)
			resRoadmapID = &u
		}
		var resApproach *string
		if res.ApproachSummary.Valid {
			resApproach = &res.ApproachSummary.String
		}
		var resCode *string
		if res.CodeSnippet.Valid {
			resCode = &res.CodeSnippet.String
		}
		attempts[i] = QuestionAttempt{
			ID:                    toUUID(res.ID),
			UserID:                toUUID(res.UserID),
			QuestionID:            toUUID(res.QuestionID),
			RoadmapID:             resRoadmapID,
			Outcome:               AttemptOutcome(res.Outcome),
			TimeSpentSecs:         res.TimeSpentSecs,
			DifficultyExperienced: PerceivedDifficulty(res.DifficultyExperienced),
			QualityRating:         res.QualityRating,
			MistakeTags:           res.MistakeTags,
			ApproachSummary:       resApproach,
			CodeSnippet:           resCode,
			CreatedAt:             res.CreatedAt.Time,
		}
	}
	return attempts, nil
}

func (r *pgxRepository) ListUserRecentAttempts(ctx context.Context, userID uuid.UUID, limit int32) ([]QuestionAttempt, error) {
	rows, err := r.q.ListUserRecentAttempts(ctx, generated.ListUserRecentAttemptsParams{
		UserID: toPgUUID(userID),
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	attempts := make([]QuestionAttempt, len(rows))
	for i, res := range rows {
		var resRoadmapID *uuid.UUID
		if res.RoadmapID.Valid {
			u := uuid.UUID(res.RoadmapID.Bytes)
			resRoadmapID = &u
		}
		var resApproach *string
		if res.ApproachSummary.Valid {
			resApproach = &res.ApproachSummary.String
		}
		var resCode *string
		if res.CodeSnippet.Valid {
			resCode = &res.CodeSnippet.String
		}
		attempts[i] = QuestionAttempt{
			ID:                    toUUID(res.ID),
			UserID:                toUUID(res.UserID),
			QuestionID:            toUUID(res.QuestionID),
			RoadmapID:             resRoadmapID,
			Outcome:               AttemptOutcome(res.Outcome),
			TimeSpentSecs:         res.TimeSpentSecs,
			DifficultyExperienced: PerceivedDifficulty(res.DifficultyExperienced),
			QualityRating:         res.QualityRating,
			MistakeTags:           res.MistakeTags,
			ApproachSummary:       resApproach,
			CodeSnippet:           resCode,
			CreatedAt:             res.CreatedAt.Time,
			QuestionTitle:         res.QuestionTitle,
			QuestionSlug:          res.QuestionSlug,
			QuestionDifficulty:    string(res.QuestionDifficulty),
		}
	}
	return attempts, nil
}

func (r *pgxRepository) GetUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) (*UserQuestionProgress, error) {
	res, err := r.q.GetUserQuestionProgress(ctx, generated.GetUserQuestionProgressParams{
		UserID:     toPgUUID(userID),
		QuestionID: toPgUUID(questionID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProgressNotFound
		}
		return nil, err
	}
	var firstSolved, lastSolved *time.Time
	if res.FirstSolvedAt.Valid {
		firstSolved = &res.FirstSolvedAt.Time
	}
	if res.LastSolvedAt.Valid {
		lastSolved = &res.LastSolvedAt.Time
	}
	var notes *string
	if res.PersonalNotes.Valid {
		notes = &res.PersonalNotes.String
	}
	return &UserQuestionProgress{
		ID:                    toUUID(res.ID),
		UserID:                toUUID(res.UserID),
		QuestionID:            toUUID(res.QuestionID),
		Status:                ProgressStatus(res.Status),
		FirstSolvedAt:         firstSolved,
		LastSolvedAt:          lastSolved,
		SolveCount:            res.SolveCount,
		IndependentSolveCount: res.IndependentSolveCount,
		TotalTimeSpentSecs:    res.TotalTimeSpentSecs,
		IsBookmarked:          res.IsBookmarked,
		PersonalNotes:         notes,
		CreatedAt:             res.CreatedAt.Time,
		UpdatedAt:             res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListUserQuestionProgress(ctx context.Context, userID uuid.UUID) ([]UserQuestionProgress, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, question_id, status, first_solved_at, last_solved_at,
		       solve_count, independent_solve_count, total_time_spent_secs,
		       is_bookmarked, personal_notes, created_at, updated_at
		FROM user_question_progress
		WHERE user_id = $1
		ORDER BY updated_at DESC`, toPgUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list user question progress: %w", err)
	}
	defer rows.Close()

	var list []UserQuestionProgress
	for rows.Next() {
		var p UserQuestionProgress
		var fSolved, lSolved *time.Time
		var notes *string
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.QuestionID, &p.Status, &fSolved, &lSolved,
			&p.SolveCount, &p.IndependentSolveCount, &p.TotalTimeSpentSecs,
			&p.IsBookmarked, &notes, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan progress row: %w", err)
		}
		p.FirstSolvedAt = fSolved
		p.LastSolvedAt = lSolved
		p.PersonalNotes = notes
		list = append(list, p)
	}
	return list, nil
}

func (r *pgxRepository) DeleteUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_question_progress WHERE user_id = $1 AND question_id = $2`, toPgUUID(userID), toPgUUID(questionID))
	if err != nil {
		return fmt.Errorf("delete user question progress: %w", err)
	}
	_, _ = r.pool.Exec(ctx, `DELETE FROM revision_schedules WHERE user_id = $1 AND question_id = $2`, toPgUUID(userID), toPgUUID(questionID))
	return nil
}

func (r *pgxRepository) UpsertUserQuestionProgress(ctx context.Context, p *UserQuestionProgress) (*UserQuestionProgress, error) {
	var firstSolved, lastSolved pgtype.Timestamptz
	if p.FirstSolvedAt != nil {
		firstSolved = pgtype.Timestamptz{Time: *p.FirstSolvedAt, Valid: true}
	}
	if p.LastSolvedAt != nil {
		lastSolved = pgtype.Timestamptz{Time: *p.LastSolvedAt, Valid: true}
	}
	var notes pgtype.Text
	if p.PersonalNotes != nil {
		notes = pgtype.Text{String: *p.PersonalNotes, Valid: true}
	}

	res, err := r.q.UpsertUserQuestionProgress(ctx, generated.UpsertUserQuestionProgressParams{
		UserID:                toPgUUID(p.UserID),
		QuestionID:            toPgUUID(p.QuestionID),
		Status:                generated.ProgressStatus(p.Status),
		FirstSolvedAt:         firstSolved,
		LastSolvedAt:          lastSolved,
		SolveCount:            p.SolveCount,
		IndependentSolveCount: p.IndependentSolveCount,
		TotalTimeSpentSecs:    p.TotalTimeSpentSecs,
		PersonalNotes:         notes,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert progress: %w", err)
	}

	var resFirstSolved, resLastSolved *time.Time
	if res.FirstSolvedAt.Valid {
		resFirstSolved = &res.FirstSolvedAt.Time
	}
	if res.LastSolvedAt.Valid {
		resLastSolved = &res.LastSolvedAt.Time
	}
	var resNotes *string
	if res.PersonalNotes.Valid {
		resNotes = &res.PersonalNotes.String
	}

	return &UserQuestionProgress{
		ID:                    toUUID(res.ID),
		UserID:                toUUID(res.UserID),
		QuestionID:            toUUID(res.QuestionID),
		Status:                ProgressStatus(res.Status),
		FirstSolvedAt:         resFirstSolved,
		LastSolvedAt:          resLastSolved,
		SolveCount:            res.SolveCount,
		IndependentSolveCount: res.IndependentSolveCount,
		TotalTimeSpentSecs:    res.TotalTimeSpentSecs,
		IsBookmarked:          res.IsBookmarked,
		PersonalNotes:         resNotes,
		CreatedAt:             res.CreatedAt.Time,
		UpdatedAt:             res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetUserProgressStats(ctx context.Context, userID uuid.UUID) (*UserProgressStats, error) {
	rows, err := r.q.GetUserProgressStats(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}
	stats := &UserProgressStats{}
	for _, row := range rows {
		switch row.Status {
		case generated.ProgressStatusSOLVED:
			stats.TotalSolved += row.Count
		case generated.ProgressStatusMASTERED:
			stats.TotalMastered += row.Count
		case generated.ProgressStatusINPROGRESS:
			stats.TotalInFlight += row.Count
		}
	}
	return stats, nil
}

func (r *pgxRepository) GetRevisionSchedule(ctx context.Context, userID, questionID uuid.UUID) (*RevisionSchedule, error) {
	res, err := r.q.GetRevisionSchedule(ctx, generated.GetRevisionScheduleParams{
		UserID:     toPgUUID(userID),
		QuestionID: toPgUUID(questionID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil if no revision scheduled yet
		}
		return nil, err
	}
	var lastReviewed *time.Time
	if res.LastReviewedAt.Valid {
		lastReviewed = &res.LastReviewedAt.Time
	}
	ef, _ := res.EaseFactor.Float64Value()
	return &RevisionSchedule{
		ID:              toUUID(res.ID),
		UserID:          toUUID(res.UserID),
		QuestionID:      toUUID(res.QuestionID),
		Stability:       res.Stability,
		Difficulty:      res.Difficulty,
		RepetitionCount: res.RepetitionCount,
		EaseFactor:      ef.Float64,
		IntervalDays:    res.IntervalDays,
		NextReviewAt:    res.NextReviewAt.Time,
		LastReviewedAt:  lastReviewed,
		CreatedAt:       res.CreatedAt.Time,
		UpdatedAt:       res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) UpsertRevisionSchedule(ctx context.Context, s *RevisionSchedule) (*RevisionSchedule, error) {
	var lastReviewed pgtype.Timestamptz
	if s.LastReviewedAt != nil {
		lastReviewed = pgtype.Timestamptz{Time: *s.LastReviewedAt, Valid: true}
	}
	var ef pgtype.Numeric
	_ = ef.Scan(fmt.Sprintf("%.2f", s.EaseFactor))

	res, err := r.q.UpsertRevisionSchedule(ctx, generated.UpsertRevisionScheduleParams{
		UserID:          toPgUUID(s.UserID),
		QuestionID:      toPgUUID(s.QuestionID),
		Stability:       s.Stability,
		Difficulty:      s.Difficulty,
		RepetitionCount: s.RepetitionCount,
		EaseFactor:      ef,
		IntervalDays:    s.IntervalDays,
		NextReviewAt:    pgtype.Timestamptz{Time: s.NextReviewAt, Valid: true},
		LastReviewedAt:  lastReviewed,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert revision: %w", err)
	}

	resEf, _ := res.EaseFactor.Float64Value()
	var resLastReviewed *time.Time
	if res.LastReviewedAt.Valid {
		resLastReviewed = &res.LastReviewedAt.Time
	}

	return &RevisionSchedule{
		ID:              toUUID(res.ID),
		UserID:          toUUID(res.UserID),
		QuestionID:      toUUID(res.QuestionID),
		Stability:       res.Stability,
		Difficulty:      res.Difficulty,
		RepetitionCount: res.RepetitionCount,
		EaseFactor:      resEf.Float64,
		IntervalDays:    res.IntervalDays,
		NextReviewAt:    res.NextReviewAt.Time,
		LastReviewedAt:  resLastReviewed,
		CreatedAt:       res.CreatedAt.Time,
		UpdatedAt:       res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListDueRevisions(ctx context.Context, userID uuid.UUID, asOf time.Time, limit, offset int32) ([]RevisionSchedule, int64, error) {
	asOfPg := pgtype.Timestamptz{Time: asOf, Valid: true}
	rows, err := r.q.ListDueRevisions(ctx, generated.ListDueRevisionsParams{
		UserID:       toPgUUID(userID),
		NextReviewAt: asOfPg,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.q.CountDueRevisions(ctx, generated.CountDueRevisionsParams{
		UserID:       toPgUUID(userID),
		NextReviewAt: asOfPg,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]RevisionSchedule, len(rows))
	for i, res := range rows {
		resEf, _ := res.EaseFactor.Float64Value()
		var resLastReviewed *time.Time
		if res.LastReviewedAt.Valid {
			resLastReviewed = &res.LastReviewedAt.Time
		}
		items[i] = RevisionSchedule{
			ID:                 toUUID(res.ID),
			UserID:             toUUID(res.UserID),
			QuestionID:         toUUID(res.QuestionID),
			Stability:          res.Stability,
			Difficulty:         res.Difficulty,
			RepetitionCount:    res.RepetitionCount,
			EaseFactor:         resEf.Float64,
			IntervalDays:       res.IntervalDays,
			NextReviewAt:       res.NextReviewAt.Time,
			LastReviewedAt:     resLastReviewed,
			QuestionTitle:      res.QuestionTitle,
			QuestionSlug:       res.QuestionSlug,
			QuestionDifficulty: string(res.QuestionDifficulty),
		}
	}
	return items, count, nil
}

func (r *pgxRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := r.queries.WithTx(tx)
	txRepo := &pgxRepository{
		pool:    r.pool,
		queries: r.queries,
		q:       txQueries,
	}

	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *pgxRepository) ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExportData, error) {
	now := time.Now().UTC()
	export := &UserExportData{
		UserID:     userID,
		Progress:   make([]UserQuestionProgress, 0),
		Attempts:   make([]QuestionAttempt, 0),
		Revisions:  make([]RevisionSchedule, 0),
		ExportedAt: now,
	}

	// 1. Progress
	progRows, err := r.pool.Query(ctx, `
		SELECT id, user_id, question_id, status, first_solved_at, last_solved_at,
		       solve_count, independent_solve_count, total_time_spent_secs,
		       is_bookmarked, personal_notes, created_at, updated_at
		FROM user_question_progress
		WHERE user_id = $1
		ORDER BY updated_at DESC`, userID)
	if err == nil {
		defer progRows.Close()
		for progRows.Next() {
			var p UserQuestionProgress
			var fSolved, lSolved *time.Time
			var notes *string
			if scanErr := progRows.Scan(
				&p.ID, &p.UserID, &p.QuestionID, &p.Status, &fSolved, &lSolved,
				&p.SolveCount, &p.IndependentSolveCount, &p.TotalTimeSpentSecs,
				&p.IsBookmarked, &notes, &p.CreatedAt, &p.UpdatedAt,
			); scanErr == nil {
				p.FirstSolvedAt = fSolved
				p.LastSolvedAt = lSolved
				p.PersonalNotes = notes
				export.Progress = append(export.Progress, p)
			}
		}
	}

	// 2. Attempts
	attRows, err := r.pool.Query(ctx, `
		SELECT a.id, a.user_id, a.question_id, a.outcome, a.time_spent_secs,
		       a.difficulty_experienced, a.quality_rating, a.mistake_tags,
		       a.approach_summary, a.code_snippet, a.created_at,
		       q.title, q.slug, q.difficulty
		FROM question_attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = $1
		ORDER BY a.created_at ASC`, userID)
	if err == nil {
		defer attRows.Close()
		for attRows.Next() {
			var a QuestionAttempt
			var appSummary, code *string
			if scanErr := attRows.Scan(
				&a.ID, &a.UserID, &a.QuestionID, &a.Outcome, &a.TimeSpentSecs,
				&a.DifficultyExperienced, &a.QualityRating, &a.MistakeTags,
				&appSummary, &code, &a.CreatedAt,
				&a.QuestionTitle, &a.QuestionSlug, &a.QuestionDifficulty,
			); scanErr == nil {
				a.ApproachSummary = appSummary
				a.CodeSnippet = code
				export.Attempts = append(export.Attempts, a)
			}
		}
	}

	// 3. Revisions
	revRows, err := r.pool.Query(ctx, `
		SELECT r.id, r.user_id, r.question_id, r.stability, r.difficulty,
		       r.repetition_count, r.ease_factor, r.interval_days,
		       r.next_review_at, r.last_reviewed_at, r.created_at, r.updated_at,
		       q.title, q.slug, q.difficulty
		FROM revision_schedules r
		JOIN questions q ON q.id = r.question_id
		WHERE r.user_id = $1
		ORDER BY r.next_review_at ASC`, userID)
	if err == nil {
		defer revRows.Close()
		for revRows.Next() {
			var s RevisionSchedule
			var lastRev *time.Time
			if scanErr := revRows.Scan(
				&s.ID, &s.UserID, &s.QuestionID, &s.Stability, &s.Difficulty,
				&s.RepetitionCount, &s.EaseFactor, &s.IntervalDays,
				&s.NextReviewAt, &lastRev, &s.CreatedAt, &s.UpdatedAt,
				&s.QuestionTitle, &s.QuestionSlug, &s.QuestionDifficulty,
			); scanErr == nil {
				s.LastReviewedAt = lastRev
				export.Revisions = append(export.Revisions, s)
			}
		}
	}

	return export, nil
}

// InMemoryRepository provides a fast in-memory implementation for unit testing.
type InMemoryRepository struct {
	mu        sync.RWMutex
	txMu      sync.Mutex
	attempts  map[uuid.UUID]QuestionAttempt
	progress  map[string]UserQuestionProgress // key: userID_questionID
	revisions map[string]RevisionSchedule     // key: userID_questionID
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		attempts:  make(map[uuid.UUID]QuestionAttempt),
		progress:  make(map[string]UserQuestionProgress),
		revisions: make(map[string]RevisionSchedule),
	}
}

func progressKey(u, q uuid.UUID) string {
	return fmt.Sprintf("%s_%s", u.String(), q.String())
}

func (m *InMemoryRepository) CreateQuestionAttempt(ctx context.Context, a *QuestionAttempt) (*QuestionAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := a.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := a.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	created := *a
	created.ID = id
	created.CreatedAt = now

	m.attempts[id] = created
	return &created, nil
}

func (m *InMemoryRepository) ListQuestionAttempts(ctx context.Context, userID, questionID uuid.UUID) ([]QuestionAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []QuestionAttempt
	for _, a := range m.attempts {
		if a.UserID == userID && a.QuestionID == questionID {
			list = append(list, a)
		}
	}
	return list, nil
}

func (m *InMemoryRepository) ListUserRecentAttempts(ctx context.Context, userID uuid.UUID, limit int32) ([]QuestionAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []QuestionAttempt
	for _, a := range m.attempts {
		if a.UserID == userID {
			list = append(list, a)
		}
	}
	if int(limit) < len(list) {
		return list[:limit], nil
	}
	return list, nil
}

func (m *InMemoryRepository) GetUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) (*UserQuestionProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.progress[progressKey(userID, questionID)]
	if !ok {
		return nil, ErrProgressNotFound
	}
	res := p
	return &res, nil
}

func (m *InMemoryRepository) ListUserQuestionProgress(ctx context.Context, userID uuid.UUID) ([]UserQuestionProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []UserQuestionProgress
	for _, p := range m.progress {
		if p.UserID == userID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *InMemoryRepository) DeleteUserQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := progressKey(userID, questionID)
	delete(m.progress, key)
	delete(m.revisions, key)
	return nil
}

func (m *InMemoryRepository) UpsertUserQuestionProgress(ctx context.Context, p *UserQuestionProgress) (*UserQuestionProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := progressKey(p.UserID, p.QuestionID)
	existing, ok := m.progress[key]

	now := time.Now().UTC()
	if !ok {
		id := p.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		created := *p
		created.ID = id
		created.CreatedAt = now
		created.UpdatedAt = now
		m.progress[key] = created
		return &created, nil
	}

	existing.SolveCount += p.SolveCount
	existing.IndependentSolveCount += p.IndependentSolveCount
	existing.TotalTimeSpentSecs += p.TotalTimeSpentSecs
	if existing.IndependentSolveCount >= 2 {
		existing.Status = StatusMastered
	} else if existing.SolveCount >= 1 {
		existing.Status = StatusSolved
	} else {
		existing.Status = p.Status
	}
	if p.PersonalNotes != nil {
		existing.PersonalNotes = p.PersonalNotes
	}
	existing.UpdatedAt = now

	m.progress[key] = existing
	res := existing
	return &res, nil
}

func (m *InMemoryRepository) GetUserProgressStats(ctx context.Context, userID uuid.UUID) (*UserProgressStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &UserProgressStats{}
	for _, p := range m.progress {
		if p.UserID == userID {
			switch p.Status {
			case StatusSolved:
				stats.TotalSolved++
			case StatusMastered:
				stats.TotalMastered++
			case StatusInProgress:
				stats.TotalInFlight++
			}
		}
	}
	return stats, nil
}

func (m *InMemoryRepository) GetRevisionSchedule(ctx context.Context, userID, questionID uuid.UUID) (*RevisionSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.revisions[progressKey(userID, questionID)]
	if !ok {
		return nil, nil
	}
	res := s
	return &res, nil
}

func (m *InMemoryRepository) UpsertRevisionSchedule(ctx context.Context, s *RevisionSchedule) (*RevisionSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := progressKey(s.UserID, s.QuestionID)
	now := time.Now().UTC()

	existing, ok := m.revisions[key]
	if !ok {
		id := s.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		created := *s
		created.ID = id
		created.CreatedAt = now
		created.UpdatedAt = now
		m.revisions[key] = created
		return &created, nil
	}

	existing.Stability = s.Stability
	existing.Difficulty = s.Difficulty
	existing.RepetitionCount = s.RepetitionCount
	existing.EaseFactor = s.EaseFactor
	existing.IntervalDays = s.IntervalDays
	existing.NextReviewAt = s.NextReviewAt
	existing.LastReviewedAt = s.LastReviewedAt
	existing.UpdatedAt = now

	m.revisions[key] = existing
	res := existing
	return &res, nil
}

func (m *InMemoryRepository) ListDueRevisions(ctx context.Context, userID uuid.UUID, asOf time.Time, limit, offset int32) ([]RevisionSchedule, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var due []RevisionSchedule
	for _, s := range m.revisions {
		if s.UserID == userID && (s.NextReviewAt.Before(asOf) || s.NextReviewAt.Equal(asOf)) {
			due = append(due, s)
		}
	}
	total := int64(len(due))
	start := int(offset)
	if start > len(due) {
		start = len(due)
	}
	end := start + int(limit)
	if end > len(due) {
		end = len(due)
	}
	return due[start:end], total, nil
}

func (m *InMemoryRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	m.txMu.Lock()
	defer m.txMu.Unlock()

	m.mu.Lock()
	backupAttempts := make(map[uuid.UUID]QuestionAttempt, len(m.attempts))
	for k, v := range m.attempts {
		backupAttempts[k] = v
	}
	backupProgress := make(map[string]UserQuestionProgress, len(m.progress))
	for k, v := range m.progress {
		backupProgress[k] = v
	}
	backupRevisions := make(map[string]RevisionSchedule, len(m.revisions))
	for k, v := range m.revisions {
		backupRevisions[k] = v
	}
	m.mu.Unlock()

	if err := fn(m); err != nil {
		m.mu.Lock()
		m.attempts = backupAttempts
		m.progress = backupProgress
		m.revisions = backupRevisions
		m.mu.Unlock()
		return err
	}
	return nil
}

func (m *InMemoryRepository) ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExportData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export := &UserExportData{
		UserID:     userID,
		Progress:   make([]UserQuestionProgress, 0),
		Attempts:   make([]QuestionAttempt, 0),
		Revisions:  make([]RevisionSchedule, 0),
		ExportedAt: time.Now().UTC(),
	}

	for _, p := range m.progress {
		if p.UserID == userID {
			export.Progress = append(export.Progress, p)
		}
	}
	for _, a := range m.attempts {
		if a.UserID == userID {
			export.Attempts = append(export.Attempts, a)
		}
	}
	for _, r := range m.revisions {
		if r.UserID == userID {
			export.Revisions = append(export.Revisions, r)
		}
	}
	return export, nil
}

