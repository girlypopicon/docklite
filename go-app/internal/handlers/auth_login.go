package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/store"
)

const (
	loginMaxAttempts  = 5
	loginWindow       = 5 * time.Minute
	delegationTTL     = 24 * time.Hour
	defaultTokenName  = "cli"
	delegationMaxAgeS = int(delegationTTL / time.Second)
)

type loginRequest struct {
	Username   string  `json:"username"`
	Password   string  `json:"password"`
	IssueToken bool    `json:"issue_token"`
	TokenName  string  `json:"token_name"`
	ExpiresAt  *string `json:"expires_at"`
}

type loginAttempt struct {
	Count        int
	FirstAttempt time.Time
}

var loginLimiter = struct {
	sync.Mutex
	Entries map[string]*loginAttempt
}{
	Entries: map[string]*loginAttempt{},
}

func (h *Handlers) AuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	contentType := r.Header.Get("Content-Type")
	expectsJSON := strings.Contains(contentType, "application/json")
	wantsHTML := strings.Contains(r.Header.Get("Accept"), "text/html")
	shouldRedirect := wantsHTML && !expectsJSON

	var req loginRequest
	if expectsJSON {
		if err := readJSON(w, r, &req); err != nil {
			respondLoginError(w, r, shouldRedirect, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) == "" {
			req.ExpiresAt = nil
		}
	} else {
		if err := r.ParseForm(); err != nil {
			respondLoginError(w, r, shouldRedirect, http.StatusBadRequest, "invalid form data")
			return
		}
		req.Username = strings.TrimSpace(r.FormValue("username"))
		req.Password = r.FormValue("password")
		req.IssueToken = parseFormBool(r.FormValue("issue_token"))
		req.TokenName = strings.TrimSpace(r.FormValue("token_name"))
		if expires := strings.TrimSpace(r.FormValue("expires_at")); expires != "" {
			req.ExpiresAt = &expires
		}
	}

	if req.Username == "" || req.Password == "" {
		respondLoginError(w, r, shouldRedirect, http.StatusBadRequest, "username and password are required")
		return
	}

	rateKey := loginRateKey(r, req.Username)
	if isRateLimited(rateKey) {
		respondLoginError(w, r, shouldRedirect, http.StatusTooManyRequests, "too many login attempts")
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		recordAttempt(rateKey)
		respondLoginError(w, r, shouldRedirect, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if err := h.store.VerifyPassword(user, req.Password); err != nil {
		recordAttempt(rateKey)
		respondLoginError(w, r, shouldRedirect, http.StatusUnauthorized, "invalid username or password")
		return
	}

	clearAttempts(rateKey)

	role := normalizeUserRole(user)
	if h.token == "" {
		respondLoginError(w, r, shouldRedirect, http.StatusInternalServerError, "delegation token not configured")
		return
	}

	delegationToken, err := createDelegationToken(user, role, h.token)
	if err != nil {
		respondLoginError(w, r, shouldRedirect, http.StatusInternalServerError, "failed to create session")
		return
	}

	setDelegationCookie(w, r, delegationToken)

	var issuedToken *store.TokenRecord
	var tokenSecret string
	if req.IssueToken {
		issuedToken, tokenSecret, err = h.issueLoginToken(user, role, req)
		if err != nil {
			respondLoginError(w, r, shouldRedirect, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if shouldRedirect {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	response := map[string]any{
		"success": true,
		"user": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"isAdmin":  user.IsAdmin == 1,
			"role":     role,
		},
	}
	if issuedToken != nil {
		response["token"] = map[string]any{
			"id":        issuedToken.ID,
			"name":      issuedToken.Name,
			"secret":    tokenSecret,
			"createdAt": issuedToken.CreatedAt,
			"expiresAt": issuedToken.ExpiresAt,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) issueLoginToken(user *store.UserRecord, role string, req loginRequest) (*store.TokenRecord, string, error) {
	tokenName := strings.TrimSpace(req.TokenName)
	if tokenName == "" {
		tokenName = defaultTokenName
	}
	var expiresAt *string
	if req.ExpiresAt != nil {
		expires := strings.TrimSpace(*req.ExpiresAt)
		if expires != "" {
			if _, err := time.Parse(time.RFC3339, expires); err != nil {
				return nil, "", errors.New("invalid expires_at")
			}
			expiresAt = &expires
		}
	}

	secret, err := generateTokenSecret()
	if err != nil {
		return nil, "", errors.New("failed to generate token")
	}
	tokenHash, fingerprint, err := hashToken(secret)
	if err != nil {
		return nil, "", errors.New("failed to hash token")
	}

	roleValue := role
	issuedFor := `{"source":"login"}`

	record := store.TokenRecord{
		Name:             tokenName,
		TokenHash:        tokenHash,
		TokenFingerprint: fingerprint,
		UserID:           &user.ID,
		Role:             &roleValue,
		ExpiresAt:        expiresAt,
		IssuedFor:        &issuedFor,
		Disabled:         0,
	}

	created, err := h.store.CreateToken(record)
	if err != nil {
		return nil, "", errors.New("failed to create token")
	}

	return created, secret, nil
}

func normalizeUserRole(user *store.UserRecord) string {
	if user == nil {
		return "user"
	}
	if user.Role != "" {
		return user.Role
	}
	if user.IsAdmin == 1 {
		return "admin"
	}
	return "user"
}

func createDelegationToken(user *store.UserRecord, role string, secret string) (string, error) {
	if user == nil {
		return "", errors.New("user missing")
	}
	claims := delegationClaims{
		UserID: user.ID,
		Role:   role,
		Admin:  role == "admin" || role == "super_admin" || user.IsAdmin == 1,
		Expiry: time.Now().Add(delegationTTL).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encoded))
	signature := mac.Sum(nil)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func setDelegationCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := shouldUseSecureCookies(r)
	http.SetCookie(w, &http.Cookie{
		Name:     delegationCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   delegationMaxAgeS,
	})
}

func shouldUseSecureCookies(r *http.Request) bool {
	if os.Getenv("DOCKLITE_INSECURE_COOKIES") == "true" {
		return false
	}
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		return false
	}
	proto = strings.Split(proto, ",")[0]
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func respondLoginError(w http.ResponseWriter, r *http.Request, shouldRedirect bool, status int, message string) {
	if shouldRedirect {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	writeError(w, status, message)
}

func loginRateKey(r *http.Request, username string) string {
	ip := clientIP(r)
	return ip + ":" + username
}

func clientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	// Proxy headers are only trustworthy when the request came from the
	// local reverse proxy; from anyone else they are attacker-controlled
	// and would let a client pick a fresh rate-limit key per request.
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return host
	}
	// nginx sets X-Real-IP to $remote_addr. In X-Forwarded-For only the
	// last entry is appended by nginx; earlier ones come from the client.
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
			return last
		}
	}
	return host
}

func parseFormBool(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "true" || value == "1" || value == "yes" {
		return true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func isRateLimited(key string) bool {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	entry, ok := loginLimiter.Entries[key]
	if !ok {
		return false
	}
	if time.Since(entry.FirstAttempt) > loginWindow {
		delete(loginLimiter.Entries, key)
		return false
	}
	return entry.Count >= loginMaxAttempts
}

func recordAttempt(key string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	now := time.Now()
	entry, ok := loginLimiter.Entries[key]
	if !ok || now.Sub(entry.FirstAttempt) > loginWindow {
		loginLimiter.Entries[key] = &loginAttempt{Count: 1, FirstAttempt: now}
		return
	}
	entry.Count++
}

func clearAttempts(key string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	delete(loginLimiter.Entries, key)
}
