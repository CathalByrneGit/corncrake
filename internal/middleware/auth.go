// Package middleware provides HTTP middleware for the EHECS API.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// Authenticate verifies the Bearer JWT and injects a ClientIdentity into the
// request context. Mirrors Revenue PAYE Modernisation's certificate-based auth.
func Authenticate(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeErrJSON(w, http.StatusUnauthorized, "UNAUTHORISED",
					"Missing or malformed Authorization header. Expected: Bearer <token>")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := jwt.MapClaims{}

			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})

			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					writeErrJSON(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Bearer token has expired.")
				} else {
					writeErrJSON(w, http.StatusUnauthorized, "INVALID_TOKEN",
						"Bearer token is invalid or could not be verified.")
				}
				return
			}

			if !token.Valid {
				writeErrJSON(w, http.StatusUnauthorized, "INVALID_TOKEN", "Bearer token is invalid.")
				return
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
