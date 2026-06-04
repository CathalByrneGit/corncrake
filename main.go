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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	_ "github.com/CathalByrneGit/corncrake/internal/tenants/ehecs"
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

	// JWT verification: prefer RS256 (JWT_PUBLIC_KEY), fall back to HS256 (JWT_SECRET)
	authCfg := buildAuthConfig()
	revStore := middleware.NewRevocationStore()
	authCfg.Revocation = revStore

	// Rate limiter: 60 req/min per IP (1 token/s) with burst of 30
	rl := middleware.NewRateLimiter(1.0, 30)

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
	r.Use(enforceHTTPS)

	// ── Unauthenticated ────────────────────────────────────────────────────
	r.Get("/corncrake/v1/health", handlers.Health)

	// ── Authenticated routes ───────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(rl.Limit())
		r.Use(middleware.Authenticate(authCfg))

		// Submission endpoints
		// Path: /{tenantID}/{holdingNumber}/{taxYear}/{quarter}/{runRef}/{subId}
		const subBase = "/corncrake/v1/{tenantID}/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}"

		r.With(middleware.RequireSoftwareParams).Post(
			subBase+"/{submissionId}", subH.Create)
		r.With(middleware.RequireSoftwareParams).Get(
			subBase+"/{submissionId}", subH.GetSubmission)
		r.With(middleware.RequireSoftwareParams).Get(
			subBase, subH.GetRun)
		r.Get("/corncrake/v1/{tenantID}/submissions/{holdingNumber}", subH.ListSubmissions)

		// Lookup endpoints — tenant-scoped; periods is survey-agnostic date math
		r.Get("/corncrake/v1/{tenantID}/lookups/periods/{holdingNumber}", handlers.GetPeriods)
		r.Get("/corncrake/v1/{tenantID}/lookups/{lookupName}", handlers.GetLookup)
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

// buildAuthConfig loads JWT verification config from environment variables.
// JWT_PUBLIC_KEY (PEM-encoded RSA public key) enables RS256.
// Falls back to HS256 with JWT_SECRET when JWT_PUBLIC_KEY is absent.
func buildAuthConfig() middleware.AuthConfig {
	if pubKeyPEM := os.Getenv("JWT_PUBLIC_KEY"); pubKeyPEM != "" {
		block, _ := pem.Decode([]byte(pubKeyPEM))
		if block == nil {
			slog.Error("JWT_PUBLIC_KEY: failed to decode PEM block")
			os.Exit(1)
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			slog.Error("JWT_PUBLIC_KEY: failed to parse key", "err", err)
			os.Exit(1)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			slog.Error("JWT_PUBLIC_KEY: key is not RSA")
			os.Exit(1)
		}
		slog.Info("JWT verification mode: RS256")
		return middleware.AuthConfig{RSAPublicKey: rsaPub}
	}
	slog.Warn("JWT verification mode: HS256 — set JWT_PUBLIC_KEY for RS256 in production")
	return middleware.AuthConfig{
		HMACSecret: []byte(envOrDefault("JWT_SECRET", "ehecs-dev-secret-replace-in-production")),
	}
}

// enforceHTTPS rejects plain HTTP in production by inspecting X-Forwarded-Proto,
// which the load balancer sets before forwarding to this service.
func enforceHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("APP_ENV") == "production" {
			if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" && proto != "https" {
				http.Error(w,
					`{"status":400,"errors":[{"code":"HTTPS_REQUIRED","message":"This API requires HTTPS."}]}`,
					http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
