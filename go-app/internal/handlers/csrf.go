package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	csrfTokenLength     = 32
	csrfHeaderName      = "X-CSRF-Token"
	csrfTokenCookieName = "X-CSRF-Token"
	csrfTokenTTL        = 24 * time.Hour
	csrfErrorMessage    = "invalid or missing CSRF token"
	csrfOriginErrorMsg  = "invalid origin"
)

type csrfToken struct {
	token     string
	expiresAt time.Time
}

// CSRF token store (in-memory, should be replaced with distributed cache in production)
var (
	csrfTokens = map[string]csrfToken{}
	csrfMu     sync.RWMutex
)

// generateCSRFToken creates a new CSRF token
func generateCSRFToken() (string, error) {
	bytes := make([]byte, csrfTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// storeCSRFToken saves a CSRF token with expiration
func storeCSRFToken(token string) {
	csrfMu.Lock()
	defer csrfMu.Unlock()
	csrfTokens[token] = csrfToken{
		token:     token,
		expiresAt: time.Now().Add(csrfTokenTTL),
	}

	// Simple cleanup of expired tokens (in production, use a background job)
	if len(csrfTokens) > 10000 {
		now := time.Now()
		for key, val := range csrfTokens {
			if now.After(val.expiresAt) {
				delete(csrfTokens, key)
			}
		}
	}
}

// validateCSRFToken checks if a CSRF token is valid
func validateCSRFToken(token string) bool {
	csrfMu.RLock()
	defer csrfMu.RUnlock()

	t, exists := csrfTokens[token]
	if !exists {
		return false
	}
	if time.Now().After(t.expiresAt) {
		return false
	}
	return true
}

// CSRFToken handler returns a new CSRF token for state-changing requests
// This is optional - Token-based API calls don't need CSRF tokens.
// However, clients that want additional protection can request a token and include it
// in the X-CSRF-Token header for state-changing requests.
func (h *Handlers) CSRFToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token, err := generateCSRFToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	storeCSRFToken(token)

	// Return token and set as header
	w.Header().Set(csrfHeaderName, token)
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"ttl":   "24h",
	})
}

// revokeCSRFToken removes a used CSRF token
func revokeCSRFToken(token string) {
	csrfMu.Lock()
	defer csrfMu.Unlock()
	delete(csrfTokens, token)
}

// validateOrigin checks if the request origin is allowed
func validateOrigin(r *http.Request) bool {
	// For API requests, check Referer header or Origin header
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	// If neither is provided but this is a state-changing request, it might be a valid same-site request
	// We'll be permissive for now but log it
	if origin == "" && referer == "" {
		return true // Could be a legitimate same-site request or token-based API call
	}

	// Validate that Origin/Referer matches request host (or forwarded host behind proxy)
	requestHost := expectedRequestHost(r)
	if origin != "" {
		originHost := hostFromURLLike(origin)
		if originHost != requestHost {
			return false
		}
	}

	if referer != "" {
		refererHost := hostFromURLLike(referer)
		if refererHost != requestHost {
			return false
		}
	}

	return true
}

func expectedRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		parts := strings.Split(forwardedHost, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return strings.TrimSpace(r.Host)
}

func hostFromURLLike(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		if parsed.Host != "" {
			return parsed.Host
		}
	}
	trimmed := strings.TrimPrefix(raw, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
}

func hasBearerAuth(r *http.Request) bool {
	if r == nil {
		return false
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return false
	}
	scheme, _, found := strings.Cut(value, " ")
	if !found {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(scheme), "Bearer")
}

func hasDelegationCookie(r *http.Request) bool {
	if r == nil {
		return false
	}
	cookie, err := r.Cookie(delegationCookieName)
	return err == nil && strings.TrimSpace(cookie.Value) != ""
}

func isSameHostRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := expectedRequestHost(r)
	if host == "" {
		return false
	}
	remoteHost, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		remoteHost = strings.TrimSpace(r.RemoteAddr)
	}
	if remoteHost == "" {
		return false
	}
	if strings.EqualFold(host, remoteHost) {
		return true
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(remoteHost))
}

func shouldEnforceCSRFFromRequest(r *http.Request) bool {
	// Cookie-authenticated browser requests should always enforce CSRF.
	if hasDelegationCookie(r) {
		return true
	}

	// Explicit bearer token requests are not cookie-authenticated CSRF targets.
	if hasBearerAuth(r) {
		return false
	}

	// Requests without bearer tokens likely rely on browser cookie auth.
	// Exempt local loopback tools without cookies for API compatibility.
	if isSameHostRequest(r) {
		return false
	}

	return true
}

// CSRFMiddleware handles CSRF token generation and validation
func CSRFMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For GET/HEAD/OPTIONS requests, generate a new CSRF token
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			token, err := generateCSRFToken()
			if err == nil {
				storeCSRFToken(token)
				w.Header().Set(csrfHeaderName, token)
				// Also set as SameSite cookie (optional, for client-side convenience)
				secure := shouldUseSecureCookies(r)
				http.SetCookie(w, &http.Cookie{
					Name:     csrfTokenCookieName,
					Value:    token,
					Path:     "/",
					MaxAge:   int(csrfTokenTTL.Seconds()),
					HttpOnly: false, // Needs to be readable by JavaScript
					SameSite: http.SameSiteLaxMode,
					Secure:   secure,
				})
			}
			next(w, r)
			return
		}

		// For state-changing requests, validate CSRF token and origin
		if r.Method == http.MethodPost || r.Method == http.MethodPut ||
			r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if !shouldEnforceCSRFFromRequest(r) {
				next(w, r)
				return
			}

			// Validate origin first
			if !validateOrigin(r) {
				writeError(w, http.StatusForbidden, csrfOriginErrorMsg)
				return
			}

			// Get CSRF token from header or form
			csrfToken := r.Header.Get(csrfHeaderName)
			if csrfToken == "" {
				// Try to get from form data
				csrfToken = r.FormValue(csrfHeaderName)
			}
			if csrfToken == "" {
				// Try to get from cookie
				if cookie, err := r.Cookie(csrfTokenCookieName); err == nil {
					csrfToken = cookie.Value
				}
			}

			if csrfToken == "" {
				writeError(w, http.StatusForbidden, csrfErrorMessage)
				return
			}

			// Validate the token
			if !validateCSRFToken(csrfToken) {
				writeError(w, http.StatusForbidden, csrfErrorMessage)
				return
			}

			// Token is valid, revoke it (one-time use)
			revokeCSRFToken(csrfToken)
		}

		next(w, r)
	}
}
