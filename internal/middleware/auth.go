// Package middleware provides HTTP middleware for the EHECS API.
package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// AuthConfig holds the configuration for JWT verification.
// Set RSAPublicKey to use RS256 (recommended for production).
// Set HMACSecret to use HS256 (development / backwards-compatibility).
// Revocation may be nil to skip JTI revocation checks.
type AuthConfig struct {
	HMACSecret   []byte
	RSAPublicKey *rsa.PublicKey
	Revocation   *RevocationStore
}

// Authenticate verifies the Bearer JWT and injects a ClientIdentity into the
// request context. Mirrors Revenue PAYE Modernisation's certificate-based auth.
func Authenticate(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				slog.Warn("AUTH_FAIL", "reason", "missing_header", "ip", ip, "path", r.URL.Path)
				writeErrJSON(w, http.StatusUnauthorized, "UNAUTHORISED",
					"Missing or malformed Authorization header. Expected: Bearer <token>")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := jwt.MapClaims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if cfg.RSAPublicKey != nil {
					if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return cfg.RSAPublicKey, nil
				}
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return cfg.HMACSecret, nil
			})

			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					slog.Warn("AUTH_FAIL", "reason", "token_expired", "ip", ip, "path", r.URL.Path)
					writeErrJSON(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Bearer token has expired.")
				} else {
					slog.Warn("AUTH_FAIL", "reason", "invalid_token", "ip", ip, "path", r.URL.Path)
					writeErrJSON(w, http.StatusUnauthorized, "INVALID_TOKEN",
						"Bearer token is invalid or could not be verified.")
				}
				return
			}

			if !token.Valid {
				slog.Warn("AUTH_FAIL", "reason", "invalid_token", "ip", ip, "path", r.URL.Path)
				writeErrJSON(w, http.StatusUnauthorized, "INVALID_TOKEN", "Bearer token is invalid.")
				return
			}

			// JTI revocation check — skipped for tokens that omit the jti claim.
			if cfg.Revocation != nil {
				if jti := stringClaim(claims, "jti"); jti != "" && cfg.Revocation.IsRevoked(jti) {
					slog.Warn("AUTH_FAIL", "reason", "token_revoked", "ip", ip, "path", r.URL.Path, "jti", jti)
					writeErrJSON(w, http.StatusUnauthorized, "TOKEN_REVOKED", "Bearer token has been revoked.")
					return
				}
			}

			client := &models.ClientIdentity{
				HoldingNumber:   stringClaim(claims, "holdingNumber"),
				SoftwareUsed:    stringClaim(claims, "softwareUsed"),
				SoftwareVersion: stringClaim(claims, "softwareVersion"),
				AgentID:         stringClaim(claims, "agentId"),
			}

			// Fallback: accept softwareUsed/softwareVersion from query params
			// (mirrors Revenue's approach)
			if client.SoftwareUsed == "" {
				client.SoftwareUsed = r.URL.Query().Get("softwareUsed")
			}
			if client.SoftwareVersion == "" {
				client.SoftwareVersion = r.URL.Query().Get("softwareVersion")
			}

			slog.Info("AUTH_SUCCESS",
				"holdingNumber", client.HoldingNumber,
				"ip", ip,
				"path", r.URL.Path,
			)

			ctx := context.WithValue(r.Context(), models.ClientContextKey{}, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSoftwareParams enforces softwareUsed and softwareVersion on submission
// endpoints — mirrors Revenue's mandatory query parameters.
func RequireSoftwareParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var missing []string
		if q.Get("softwareUsed") == "" {
			missing = append(missing, "softwareUsed")
		}
		if q.Get("softwareVersion") == "" {
			missing = append(missing, "softwareVersion")
		}
		if len(missing) > 0 {
			writeErrJSON(w, http.StatusBadRequest, "MISSING_QUERY_PARAMS",
				"Required query parameters missing: "+strings.Join(missing, ", "))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func stringClaim(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

type errBody struct {
	Status int `json:"status"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func writeErrJSON(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b := errBody{Status: status}
	b.Errors = append(b.Errors, struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
	_ = json.NewEncoder(w).Encode(b)
}
