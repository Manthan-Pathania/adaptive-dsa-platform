package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/adaptive-dsa/platform/internal/adaptive"
	"github.com/adaptive-dsa/platform/internal/ai"
	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/payments"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/adaptive-dsa/platform/internal/platform/middleware"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/go-chi/chi/v5"
)

// Server coordinates the HTTP transport layer and health endpoints.
type Server struct {
	cfg             *config.Config
	logger          *slog.Logger
	db              database.Checker
	identityService identity.Service
	catalogService  catalog.Service
	progressService progress.Service
	creditsService  credits.Service
	aiService       ai.Service
	adaptiveService adaptive.Service
	paymentsService payments.Service
	httpServer      *http.Server
	startTime       time.Time
}

// NewServer configures a new HTTP Server instance.
func NewServer(
	cfg *config.Config,
	log *slog.Logger,
	db database.Checker,
	identitySvc identity.Service,
	catalogSvc catalog.Service,
	progressSvc progress.Service,
	creditsSvc credits.Service,
	aiSvc ai.Service,
	adaptiveSvc adaptive.Service,
	paymentsSvc payments.Service,
) *Server {
	if log == nil {
		log = slog.Default()
	}

	s := &Server{
		cfg:             cfg,
		logger:          log,
		db:              db,
		identityService: identitySvc,
		catalogService:  catalogSvc,
		progressService: progressSvc,
		creditsService:  creditsSvc,
		aiService:       aiSvc,
		adaptiveService: adaptiveSvc,
		paymentsService: paymentsSvc,
		startTime:       time.Now(),
	}

	r := chi.NewRouter()

	// Platform middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))

	isProd := false
	maxBody := int64(1 << 20)
	var allowedOrigins []string
	if s.cfg != nil {
		isProd = s.cfg.AppEnv == config.EnvProduction
		maxBody = s.cfg.Server.MaxBodyBytes
		allowedOrigins = s.cfg.Server.AllowedOriginsList()
	}
	r.Use(middleware.SecurityHeaders(isProd))
	r.Use(middleware.BodyLimit(maxBody))
	if len(allowedOrigins) > 0 {
		r.Use(middleware.CORS(allowedOrigins))
	}

	// Health and probe endpoints
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	// API v1 route tree
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "pong"})
		})

		cookieName := "session_token"
		csrfSecret := "super-secret-csrf-key-min-32-bytes!"
		if s.cfg != nil {
			cookieName = s.cfg.Session.CookieName
			csrfSecret = s.cfg.Auth.CSRFSecret
		}

		var authMw func(http.Handler) http.Handler
		if s.identityService != nil {
			authMw = identity.Authenticate(s.identityService, cookieName)
			csrfMw := identity.RequireCSRF(csrfSecret)

			authHandler := identity.NewHandler(s.identityService, s.cfg)
			authHandler.RegisterRoutes(r, authMw, csrfMw)

			// Admin-only test/probe and metrics route verifying RBAC boundary
			r.Route("/admin", func(r chi.Router) {
				r.Use(authMw)
				r.Use(identity.RequireAuth)
				r.Use(identity.RequireRole(identity.RoleAdmin))

				r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]string{
						"status":  "admin_authorized",
						"message": "welcome to the admin boundary",
					})
				})

				r.Get("/metrics", s.handleAdminMetrics)
			})
		}

		// Mount Catalog routes (questions, topics, roadmaps, admin question bank)
		if s.catalogService != nil {
			appEnv := "development"
			if s.cfg != nil {
				appEnv = s.cfg.AppEnv
			}
			catalogHandler := catalog.NewHandler(s.catalogService, log, appEnv)
			catalogHandler.RegisterRoutes(r, authMw)
			if authMw != nil {
				catalogHandler.RegisterAdminRoutes(r, authMw, identity.RequireAuth, identity.RequireRole(identity.RoleAdmin))
			}
		}

		// Mount Progress & Revision routes
		if s.progressService != nil && authMw != nil {
			progressHandler := progress.NewHandler(s.progressService, log)
			progressHandler.RegisterRoutes(r, authMw, identity.RequireAuth)
		}

		// Mount Credits & Billing routes
		if s.creditsService != nil && authMw != nil {
			creditsHandler := credits.NewHandler(s.creditsService, log)
			creditsHandler.RegisterRoutes(r, authMw, identity.RequireAuth)
			creditsHandler.RegisterAdminRoutes(r, authMw, identity.RequireAuth, identity.RequireRole(identity.RoleAdmin))
		}

		// Mount AI Tutoring routes
		if s.aiService != nil && authMw != nil {
			aiHandler := ai.NewHandler(s.aiService, log, s.identityService)
			aiHandler.RegisterRoutes(r, authMw, identity.RequireAuth)
		}

		// Mount Adaptive Preparation Engine routes
		if s.adaptiveService != nil && authMw != nil {
			adaptiveHandler := adaptive.NewHandler(s.adaptiveService, log)
			adaptiveHandler.RegisterRoutes(r, authMw, identity.RequireAuth)
		}

		// Mount Payments & Webhooks routes
		if s.paymentsService != nil && authMw != nil {
			csrfMw := identity.RequireCSRF(csrfSecret)
			paymentsHandler := payments.NewHandler(s.paymentsService, log)
			paymentsHandler.RegisterRoutes(r, authMw, identity.RequireAuth, csrfMw, identity.RequireRole(identity.RoleAdmin))
		}
	})

	s.httpServer = &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return s
}

// Start begins listening for incoming HTTP requests.
func (s *Server) Start() error {
	s.logger.Info("starting http server",
		"address", s.cfg.Server.Address(),
		"env", s.cfg.AppEnv,
		"read_timeout", s.cfg.Server.ReadTimeout,
		"write_timeout", s.cfg.Server.WriteTimeout,
	)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen failed: %w", err)
	}

	return nil
}

// Shutdown initiates graceful termination of the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server")
	return s.httpServer.Shutdown(ctx)
}

// Handler returns the underlying http.Handler for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// LivenessResponse models the /healthz payload.
type LivenessResponse struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	Timestamp     string `json:"timestamp"`
}

// handleHealthz responds immediately to indicate the process is alive.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := LivenessResponse{
		Status:        "healthy",
		UptimeSeconds: int64(time.Since(s.startTime).Seconds()),
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ReadinessResponse models the /readyz payload.
type ReadinessResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Checks    map[string]interface{} `json:"checks"`
}

// handleReadyz verifies database connectivity and pool health before reporting readiness.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]interface{})
	isReady := true

	if s.db == nil {
		isReady = false
		checks["database"] = map[string]interface{}{
			"available": false,
			"message":   "database dependency not configured",
		}
	} else {
		dbHealth := s.db.Health(r.Context())
		checks["database"] = map[string]interface{}{
			"available":      dbHealth.Available,
			"latency_ms":     float64(dbHealth.Latency.Microseconds()) / 1000.0,
			"total_conns":    dbHealth.TotalConns,
			"idle_conns":     dbHealth.IdleConns,
			"acquired_conns": dbHealth.AcquiredConns,
			"max_conns":      dbHealth.MaxConns,
			"message":        dbHealth.Message,
		}

		if !dbHealth.Available {
			isReady = false
		}
	}

	resp := ReadinessResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}

	w.Header().Set("Content-Type", "application/json")
	if isReady {
		resp.Status = "ready"
		w.WriteHeader(http.StatusOK)
	} else {
		resp.Status = "unavailable"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// handleAdminMetrics provides structured operational diagnostics for administrators.
func (s *Server) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := map[string]any{
		"app_env":        s.cfg.AppEnv,
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"runtime": map[string]any{
			"goroutines":       runtime.NumGoroutine(),
			"heap_alloc_bytes": memStats.HeapAlloc,
			"heap_sys_bytes":   memStats.HeapSys,
			"num_gc":           memStats.NumGC,
		},
	}

	if s.db != nil {
		dbHealth := s.db.Health(r.Context())
		metrics["database"] = map[string]any{
			"available":      dbHealth.Available,
			"latency_ms":     float64(dbHealth.Latency.Microseconds()) / 1000.0,
			"total_conns":    dbHealth.TotalConns,
			"idle_conns":     dbHealth.IdleConns,
			"acquired_conns": dbHealth.AcquiredConns,
			"max_conns":      dbHealth.MaxConns,
		}
	}

	if s.paymentsService != nil {
		if stats, err := s.paymentsService.GetJobStats(r.Context()); err == nil {
			metrics["background_jobs"] = stats
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(metrics)
}
