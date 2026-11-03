package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX models the query execution interface fulfilled by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Job struct {
	ID          uuid.UUID       `json:"id"`
	JobType     string          `json:"job_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"` // 'PENDING', 'RUNNING', 'COMPLETED', 'FAILED'
	RetryCount  int32           `json:"retry_count"`
	MaxRetries  int32           `json:"max_retries"`
	LastError   *string         `json:"last_error,omitempty"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Repository defines database operations for payments, webhook idempotency, and background jobs.
type Repository interface {
	CreateOrder(ctx context.Context, order *PaymentOrder) (*PaymentOrder, error)
	GetOrderByID(ctx context.Context, id uuid.UUID) (*PaymentOrder, error)
	GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (*PaymentOrder, error)
	GetOrderByRazorpayOrderID(ctx context.Context, rpOrderID string) (*PaymentOrder, error)
	GetOrderByRazorpayOrderIDForUpdate(ctx context.Context, rpOrderID string) (*PaymentOrder, error)
	GetOrderByIdempotencyKey(ctx context.Context, key string) (*PaymentOrder, error)
	UpdateOrder(ctx context.Context, order *PaymentOrder) error
	ListOrdersByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]PaymentOrder, int64, error)
	ListAllOrders(ctx context.Context, limit, offset int32) ([]PaymentOrder, int64, error)

	RecordWebhookEvent(ctx context.Context, event *WebhookEvent) (bool, error)
	UpdateWebhookEventStatus(ctx context.Context, eventID string, status string, errMsg string) error

	EnqueueJob(ctx context.Context, jobType string, payload any) error
	FetchPendingJobs(ctx context.Context, limit int) ([]Job, error)
	UpdateJob(ctx context.Context, job *Job) error
	GetJobStats(ctx context.Context) (map[string]int64, error)

	ExecTx(ctx context.Context, fn func(Repository) error) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
	db   DBTX
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
		db:   pool,
	}
}

func (r *PostgresRepository) withTx(tx pgx.Tx) *PostgresRepository {
	return &PostgresRepository{
		pool: r.pool,
		db:   tx,
	}
}

func (r *PostgresRepository) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("payments: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txRepo := r.withTx(tx)
	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("payments: commit tx: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CreateOrder(ctx context.Context, order *PaymentOrder) (*PaymentOrder, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now

	var rpOrderID *string
	if order.RazorpayOrderID != "" {
		rpOrderID = &order.RazorpayOrderID
	}
	var idempotencyKey *string
	if order.IdempotencyKey != "" {
		idempotencyKey = &order.IdempotencyKey
	}

	query := `
		INSERT INTO payment_orders (
			id, user_id, package_id, amount_paise, currency, credits_amount,
			status, razorpay_order_id, idempotency_key, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, user_id, package_id, amount_paise, currency, credits_amount,
		          status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		          idempotency_key, created_at, updated_at
	`

	var out PaymentOrder
	var dbRpOrder, dbRpPayment, dbRpSig, dbIdemp *string
	var dbStatus string

	err := r.db.QueryRow(ctx, query,
		order.ID, order.UserID, order.PackageID, order.AmountPaise, order.Currency, order.CreditsAmount,
		string(order.Status), rpOrderID, idempotencyKey, order.CreatedAt, order.UpdatedAt,
	).Scan(
		&out.ID, &out.UserID, &out.PackageID, &out.AmountPaise, &out.Currency, &out.CreditsAmount,
		&dbStatus, &dbRpOrder, &dbRpPayment, &dbRpSig,
		&dbIdemp, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("payments: insert order: %w", err)
	}

	out.Status = PaymentOrderStatus(dbStatus)
	if dbRpOrder != nil {
		out.RazorpayOrderID = *dbRpOrder
	}
	if dbRpPayment != nil {
		out.RazorpayPaymentID = *dbRpPayment
	}
	if dbRpSig != nil {
		out.RazorpaySignature = *dbRpSig
	}
	if dbIdemp != nil {
		out.IdempotencyKey = *dbIdemp
	}

	return &out, nil
}

func (r *PostgresRepository) scanOrder(row pgx.Row) (*PaymentOrder, error) {
	var out PaymentOrder
	var dbRpOrder, dbRpPayment, dbRpSig, dbIdemp *string
	var dbStatus string

	err := row.Scan(
		&out.ID, &out.UserID, &out.PackageID, &out.AmountPaise, &out.Currency, &out.CreditsAmount,
		&dbStatus, &dbRpOrder, &dbRpPayment, &dbRpSig,
		&dbIdemp, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("payments: scan order: %w", err)
	}

	out.Status = PaymentOrderStatus(dbStatus)
	if dbRpOrder != nil {
		out.RazorpayOrderID = *dbRpOrder
	}
	if dbRpPayment != nil {
		out.RazorpayPaymentID = *dbRpPayment
	}
	if dbRpSig != nil {
		out.RazorpaySignature = *dbRpSig
	}
	if dbIdemp != nil {
		out.IdempotencyKey = *dbIdemp
	}

	return &out, nil
}

func (r *PostgresRepository) GetOrderByID(ctx context.Context, id uuid.UUID) (*PaymentOrder, error) {
	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE id = $1
	`
	return r.scanOrder(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresRepository) GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (*PaymentOrder, error) {
	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE id = $1
		FOR UPDATE
	`
	return r.scanOrder(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresRepository) GetOrderByRazorpayOrderID(ctx context.Context, rpOrderID string) (*PaymentOrder, error) {
	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE razorpay_order_id = $1
	`
	return r.scanOrder(r.db.QueryRow(ctx, query, rpOrderID))
}

func (r *PostgresRepository) GetOrderByRazorpayOrderIDForUpdate(ctx context.Context, rpOrderID string) (*PaymentOrder, error) {
	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE razorpay_order_id = $1
		FOR UPDATE
	`
	return r.scanOrder(r.db.QueryRow(ctx, query, rpOrderID))
}

func (r *PostgresRepository) GetOrderByIdempotencyKey(ctx context.Context, key string) (*PaymentOrder, error) {
	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE idempotency_key = $1
	`
	return r.scanOrder(r.db.QueryRow(ctx, query, key))
}

func (r *PostgresRepository) UpdateOrder(ctx context.Context, order *PaymentOrder) error {
	order.UpdatedAt = time.Now().UTC()
	var rpOrderID, rpPaymentID, rpSig *string
	if order.RazorpayOrderID != "" {
		rpOrderID = &order.RazorpayOrderID
	}
	if order.RazorpayPaymentID != "" {
		rpPaymentID = &order.RazorpayPaymentID
	}
	if order.RazorpaySignature != "" {
		rpSig = &order.RazorpaySignature
	}

	query := `
		UPDATE payment_orders
		SET status = $1,
		    razorpay_order_id = $2,
		    razorpay_payment_id = $3,
		    razorpay_signature = $4,
		    updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query,
		string(order.Status), rpOrderID, rpPaymentID, rpSig, order.UpdatedAt, order.ID,
	)
	if err != nil {
		return fmt.Errorf("payments: update order: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListOrdersByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]PaymentOrder, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payment_orders WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("payments: count user orders: %w", err)
	}

	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("payments: list user orders: %w", err)
	}
	defer rows.Close()

	var orders []PaymentOrder
	for rows.Next() {
		var out PaymentOrder
		var dbRpOrder, dbRpPayment, dbRpSig, dbIdemp *string
		var dbStatus string

		err := rows.Scan(
			&out.ID, &out.UserID, &out.PackageID, &out.AmountPaise, &out.Currency, &out.CreditsAmount,
			&dbStatus, &dbRpOrder, &dbRpPayment, &dbRpSig,
			&dbIdemp, &out.CreatedAt, &out.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("payments: scan order row: %w", err)
		}
		out.Status = PaymentOrderStatus(dbStatus)
		if dbRpOrder != nil {
			out.RazorpayOrderID = *dbRpOrder
		}
		if dbRpPayment != nil {
			out.RazorpayPaymentID = *dbRpPayment
		}
		if dbRpSig != nil {
			out.RazorpaySignature = *dbRpSig
		}
		if dbIdemp != nil {
			out.IdempotencyKey = *dbIdemp
		}
		orders = append(orders, out)
	}

	return orders, total, nil
}

func (r *PostgresRepository) ListAllOrders(ctx context.Context, limit, offset int32) ([]PaymentOrder, int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payment_orders`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("payments: count all orders: %w", err)
	}

	query := `
		SELECT id, user_id, package_id, amount_paise, currency, credits_amount,
		       status, razorpay_order_id, razorpay_payment_id, razorpay_signature,
		       idempotency_key, created_at, updated_at
		FROM payment_orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("payments: list all orders: %w", err)
	}
	defer rows.Close()

	var orders []PaymentOrder
	for rows.Next() {
		var out PaymentOrder
		var dbRpOrder, dbRpPayment, dbRpSig, dbIdemp *string
		var dbStatus string

		err := rows.Scan(
			&out.ID, &out.UserID, &out.PackageID, &out.AmountPaise, &out.Currency, &out.CreditsAmount,
			&dbStatus, &dbRpOrder, &dbRpPayment, &dbRpSig,
			&dbIdemp, &out.CreatedAt, &out.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("payments: scan order row: %w", err)
		}
		out.Status = PaymentOrderStatus(dbStatus)
		if dbRpOrder != nil {
			out.RazorpayOrderID = *dbRpOrder
		}
		if dbRpPayment != nil {
			out.RazorpayPaymentID = *dbRpPayment
		}
		if dbRpSig != nil {
			out.RazorpaySignature = *dbRpSig
		}
		if dbIdemp != nil {
			out.IdempotencyKey = *dbIdemp
		}
		orders = append(orders, out)
	}

	return orders, total, nil
}

func (r *PostgresRepository) RecordWebhookEvent(ctx context.Context, event *WebhookEvent) (bool, error) {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	event.CreatedAt = time.Now().UTC()

	query := `
		INSERT INTO payment_webhook_events (id, event_id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO NOTHING
	`
	cmd, err := r.db.Exec(ctx, query, event.ID, event.EventID, event.EventType, event.Payload, event.Status, event.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("payments: insert webhook event: %w", err)
	}

	return cmd.RowsAffected() > 0, nil
}

func (r *PostgresRepository) UpdateWebhookEventStatus(ctx context.Context, eventID string, status string, errMsg string) error {
	now := time.Now().UTC()
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}

	query := `
		UPDATE payment_webhook_events
		SET status = $1, error_message = $2, processed_at = $3
		WHERE event_id = $4
	`
	_, err := r.db.Exec(ctx, query, status, errPtr, now, eventID)
	if err != nil {
		return fmt.Errorf("payments: update webhook status: %w", err)
	}
	return nil
}

func (r *PostgresRepository) EnqueueJob(ctx context.Context, jobType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payments: marshal job payload: %w", err)
	}

	query := `
		INSERT INTO background_jobs (id, job_type, payload, status, retry_count, max_retries, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'PENDING', 0, 3, NOW(), NOW(), NOW())
	`
	_, err = r.db.Exec(ctx, query, uuid.New(), jobType, payloadBytes)
	if err != nil {
		return fmt.Errorf("payments: enqueue job: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FetchPendingJobs(ctx context.Context, limit int) ([]Job, error) {
	query := `
		WITH claimable AS (
			SELECT id
			FROM background_jobs
			WHERE (status = 'PENDING' AND scheduled_at <= NOW())
			   OR (status = 'RUNNING' AND updated_at < NOW() - INTERVAL '5 minutes')
			ORDER BY scheduled_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE background_jobs j
		SET status = 'RUNNING', updated_at = NOW()
		FROM claimable
		WHERE j.id = claimable.id
		RETURNING j.id, j.job_type, j.payload, j.status, j.retry_count, j.max_retries, j.last_error, j.scheduled_at, j.created_at, j.updated_at
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("payments: fetch pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		err := rows.Scan(
			&j.ID, &j.JobType, &j.Payload, &j.Status, &j.RetryCount, &j.MaxRetries,
			&j.LastError, &j.ScheduledAt, &j.CreatedAt, &j.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("payments: scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *PostgresRepository) UpdateJob(ctx context.Context, job *Job) error {
	job.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE background_jobs
		SET status = $1, retry_count = $2, last_error = $3, scheduled_at = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query, job.Status, job.RetryCount, job.LastError, job.ScheduledAt, job.UpdatedAt, job.ID)
	if err != nil {
		return fmt.Errorf("payments: update job: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetJobStats(ctx context.Context) (map[string]int64, error) {
	query := `SELECT status, COUNT(*) FROM background_jobs GROUP BY status`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("payments: get job stats: %w", err)
	}
	defer rows.Close()

	stats := map[string]int64{
		"PENDING":   0,
		"RUNNING":   0,
		"COMPLETED": 0,
		"FAILED":    0,
	}

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("payments: scan job stat: %w", err)
		}
		stats[status] = count
	}

	return stats, nil
}
