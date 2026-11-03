package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adaptive-dsa/platform/internal/adaptive"
	"github.com/adaptive-dsa/platform/internal/ai"
	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/catalog/importer"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/payments"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/adaptive-dsa/platform/internal/platform/logger"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/adaptive-dsa/platform/internal/server"
	"github.com/google/uuid"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration failure: %w", err)
	}

	// 2. Initialize structured logging
	log := logger.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	ctx := context.Background()

	log.InfoContext(ctx, "initializing application",
		"app_env", cfg.AppEnv,
		"server_address", cfg.Server.Address(),
		"db_url", cfg.Database.RedactedDatabaseURL(),
		"log_level", cfg.Log.Level,
		"log_format", cfg.Log.Format,
	)

	// 3. Initialize PostgreSQL connection pool
	var dbChecker database.Checker
	var identitySvc identity.Service
	var catalogSvc catalog.Service
	var progressSvc progress.Service
	var creditsSvc credits.Service
	var aiSvc ai.Service
	var adaptiveSvc adaptive.Service
	var paymentsSvc payments.Service

	db, err := database.New(ctx, cfg.Database, log)
	if err != nil {
		// Log database failure. In development/testing, allow the server process to start so
		// /readyz accurately reports HTTP 503 and orchestration health checks function.
		log.WarnContext(ctx, "database connection could not be established at startup",
			"error", err.Error(),
			"note", "server will start, but /readyz will report service unavailable until database connects",
		)
	} else {
		dbChecker = db
		defer db.Close()

		identityRepo := identity.NewSQLRepository(db.Pool())
		identitySvc = identity.NewService(identityRepo, cfg, log)

		catalogRepo := catalog.NewPostgresRepository(db.Pool())
		catalogImp := importer.New(catalogRepo)
		catalogSvc = catalog.NewService(catalogRepo, catalogImp)

		progressRepo := progress.NewPostgresRepository(db.Pool())
		progressSvc = progress.NewService(progressRepo)

		creditsRepo := credits.NewPostgresRepository(db.Pool())
		creditsSvc = credits.NewService(creditsRepo)

		if identitySvc != nil && creditsSvc != nil {
			identitySvc.SetOnUserRegistered(func(ctx context.Context, userID uuid.UUID) error {
				welcomeAmount := int32(50)
				if cfg != nil && cfg.AI.WelcomeCredits > 0 {
					welcomeAmount = cfg.AI.WelcomeCredits
				}
				_, err := creditsSvc.EnsureAccount(ctx, userID, welcomeAmount)
				return err
			})
		}

		aiRepo := ai.NewPostgresRepository(db.Pool())
		var aiProvider ai.AIProvider
		useMockAI := os.Getenv("USE_MOCK_AI") == "true"
		if !useMockAI && cfg.AI.GeminiAPIKey != "" {
			log.InfoContext(ctx, "initializing Google Gemini AI tutor provider", "model", cfg.AI.GeminiModel)
			aiProvider = ai.NewGeminiProvider(cfg.AI.GeminiAPIKey, cfg.AI.GeminiModel, cfg.AI.GeminiTimeout)
		} else {
			log.InfoContext(ctx, "using Mock AI Provider for deterministic local evaluation")
			aiProvider = ai.NewMockAIProvider()
		}
		aiSvc = ai.NewService(aiRepo, aiProvider, creditsSvc, catalogSvc)

		adaptiveRepo := adaptive.NewPostgresRepository(db.Pool())
		adaptiveSvc = adaptive.NewService(adaptiveRepo, progressSvc, catalogSvc)

		paymentsRepo := payments.NewPostgresRepository(db.Pool())
		var paymentProvider payments.PaymentProvider
		if cfg.Payments.RazorpayKeyID != "" && cfg.Payments.RazorpayKeySecret != "" && cfg.Payments.RazorpayKeyID != "rzp_test_placeholder_key_id" {
			log.InfoContext(ctx, "initializing Razorpay payment gateway provider (test mode)")
			paymentProvider = payments.NewRazorpayProvider(
				cfg.Payments.RazorpayKeyID,
				cfg.Payments.RazorpayKeySecret,
				cfg.Payments.RazorpayWebhookSecret,
				10*time.Second,
			)
		} else {
			log.InfoContext(ctx, "Razorpay credentials not configured; using Mock Payment Provider for offline development")
			paymentProvider = payments.NewMockPaymentProvider()
		}
		paymentsSvc = payments.NewService(paymentsRepo, paymentProvider, creditsSvc, log)

		// Start durable background payment worker with cancellable lifecycle context
		workerCtx, workerCancel := context.WithCancel(context.Background())
		defer workerCancel()
		paymentWorker := payments.NewWorker(paymentsRepo, log)
		go paymentWorker.Start(workerCtx)
	}

	// 4. Initialize HTTP server
	srv := server.NewServer(cfg, log, dbChecker, identitySvc, catalogSvc, progressSvc, creditsSvc, aiSvc, adaptiveSvc, paymentsSvc)

	// Channel to capture critical server runtime errors
	serverErrors := make(chan error, 1)

	// 5. Start HTTP server in background goroutine
	go func() {
		log.InfoContext(ctx, "http server listening", "address", cfg.Server.Address())
		if err := srv.Start(); err != nil {
			serverErrors <- err
		}
	}()

	// 6. Listen for OS termination signals (SIGINT, SIGTERM)
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("http server failure: %w", err)

	case sig := <-shutdownSignals:
		log.InfoContext(ctx, "shutdown signal received, initiating graceful termination",
			"signal", sig.String(),
			"timeout", cfg.Server.ShutdownTimeout,
		)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.ErrorContext(ctx, "graceful shutdown encountered error, forcing exit", "error", err.Error())
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		log.InfoContext(ctx, "application terminated cleanly")
		return nil
	}
}
