// Corncrake Submission REST API
// Central Statistics Office, Ireland
//
// Open source — MIT Licence
// Repository: https://github.com/CathalByrneGit/corncrake
//
// Modelled on Revenue Ireland's PAYE Modernisation REST API architecture.
// Built with: chi (router), golang-jwt (auth), google/uuid (idempotency keys)
// Logging: log/slog (stdlib, no external dep)
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/CathalByrneGit/corncrake/internal/handlers"
	"github.com/CathalByrneGit/corncrake/internal/middleware"
	"github.com/CathalByrneGit/corncrake/internal/services"
)

func main() {
	// Structured JSON logging in production; text in development
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	port := envOrDefault("PORT", "3000")
	secret := envOrDefault("JWT_SECRET", "ehecs-dev-secret-replace-in-production")

	// Select store backend from DB_DRIVER env var: memory | postgres | sqlserver
	store, err := services.NewStore(services.DBConfigFromEnv())
	if err != nil {
		slog.Error("failed to initialise store", "err", err)
		os.Exit(1)
	}
	subH := &handlers.SubmissionHandler{Store: store}

	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(requestLogger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(securityHeaders)

	// ── Unauthenticated ────────────────────────────────────────────────────
	r.Get("/corncrake/v1/health", handlers.Health)

	// ── Authenticated routes ───────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(secret))

		// Submission endpoints
		// Path mirrors Revenue: /{holdingNumber}/{taxYear}/{quarter}/{runRef}/{subId}
		const subBase = "/corncrake/v1/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}"

		r.With(middleware.RequireSoftwareParams).Post(
			subBase+"/{submissionId}", subH.Create)
		r.With(middleware.RequireSoftwareParams).Get(
			subBase+"/{submissionId}", subH.GetSubmission)
		r.With(middleware.RequireSoftwareParams).Get(
			subBase, subH.GetRun)
		r.Get("/corncrake/v1/submissions/{holdingNumber}", subH.ListSubmissions)

		// Lookup endpoints
		r.Get("/corncrake/v1/lookups/occupation-codes", handlers.GetOccupationCodes)
		r.Get("/corncrake/v1/lookups/periods/{holdingNumber}", handlers.GetPeriods)
		r.Get("/corncrake/v1/lookups/schema-version", handlers.GetSchemaVersion)
	})

	// ── 404 handler ────────────────────────────────────────────────────────
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handlers.JSONError(w, http.StatusNotFound, "NOT_FOUND",
			fmt.Sprintf("No route found for %s %s", r.Method, r.URL.Path))
	})

	addr := ":" + port
	slog.Info("Corncrake API starting",
		"addr", addr,
		"env", envOrDefault("APP_ENV", "development"),
	)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("Server failed", "err", err)
		os.Exit(1)
	}
}

// requestLogger is a minimal structured request logger using slog.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"latency", time.Since(start).String(),
			"reqId", chimiddleware.GetReqID(r.Context()),
		)
	})
}

// securityHeaders adds minimal security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
