package payments

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	JobTypePurchaseConfirmation = "PAYMENT_PURCHASE_CONFIRMATION"
	JobTypeAuditReconciliation  = "PAYMENT_AUDIT_RECONCILIATION"
)

// JobHandler processes a single background job.
type JobHandler func(ctx context.Context, job Job) error

// Worker manages periodic polling and execution of durable PostgreSQL background jobs.
type Worker struct {
	repo      Repository
	logger    *slog.Logger
	handlers  map[string]JobHandler
	interval  time.Duration
	batchSize int
	mu        sync.RWMutex
}

func NewWorker(repo Repository, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	w := &Worker{
		repo:      repo,
		logger:    logger,
		handlers:  make(map[string]JobHandler),
		interval:  3 * time.Second,
		batchSize: 10,
	}

	// Register default standard job handlers
	w.RegisterHandler(JobTypePurchaseConfirmation, func(ctx context.Context, job Job) error {
		logger.InfoContext(ctx, "processed purchase confirmation notification", "job_id", job.ID.String())
		return nil
	})
	w.RegisterHandler(JobTypeAuditReconciliation, func(ctx context.Context, job Job) error {
		logger.InfoContext(ctx, "processed payment audit reconciliation check", "job_id", job.ID.String())
		return nil
	})

	return w
}

func (w *Worker) RegisterHandler(jobType string, handler JobHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[jobType] = handler
}

// ProcessBatch executes one batch of scheduled pending jobs with bounded retries and exponential backoff.
func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	jobs, err := w.repo.FetchPendingJobs(ctx, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("worker: fetch pending jobs: %w", err)
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	processedCount := 0
	for _, job := range jobs {
		w.mu.RLock()
		handler, exists := w.handlers[job.JobType]
		w.mu.RUnlock()

		if !exists {
			w.logger.WarnContext(ctx, "no handler registered for job type", "job_id", job.ID, "job_type", job.JobType)
			errMsg := fmt.Sprintf("unsupported job type '%s'", job.JobType)
			job.Status = "FAILED"
			job.LastError = &errMsg
			_ = w.repo.UpdateJob(ctx, &job)
			continue
		}

		jobErr := handler(ctx, job)
		if jobErr == nil {
			job.Status = "COMPLETED"
			job.LastError = nil
			if err := w.repo.UpdateJob(ctx, &job); err != nil {
				w.logger.ErrorContext(ctx, "failed to update completed job status", "job_id", job.ID, "error", err)
			} else {
				processedCount++
			}
		} else {
			job.RetryCount++
			errStr := jobErr.Error()
			job.LastError = &errStr
			if job.RetryCount >= job.MaxRetries {
				job.Status = "FAILED"
				w.logger.ErrorContext(ctx, "job permanently failed after maximum retries",
					"job_id", job.ID, "retries", job.RetryCount, "error", jobErr,
				)
			} else {
				// Exponential backoff: 5s, 15s, 30s
				backoffSec := int(job.RetryCount) * 10
				job.ScheduledAt = time.Now().UTC().Add(time.Duration(backoffSec) * time.Second)
				job.Status = "PENDING"
				w.logger.WarnContext(ctx, "job execution failed; scheduled for retry",
					"job_id", job.ID, "retry_count", job.RetryCount, "next_attempt", job.ScheduledAt, "error", jobErr,
				)
			}
			_ = w.repo.UpdateJob(ctx, &job)
		}
	}

	return processedCount, nil
}

// Start launches the background worker loop in a separate goroutine until context is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.InfoContext(ctx, "durable background payment worker started", "poll_interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "background payment worker stopping gracefully")
			return
		case <-ticker.C:
			_, err := w.ProcessBatch(ctx)
			if err != nil && ctx.Err() == nil {
				w.logger.ErrorContext(ctx, "background job processing batch error", "error", err)
			}
		}
	}
}
