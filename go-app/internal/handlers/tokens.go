package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/store"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory       = 64 * 1024
	argonIterations   = 3
	argonParallelism  = 2
	argonKeyLength    = 32
	argonSaltLength   = 16
	defaultTokenTTL   = 90 * 24 * time.Hour // 90 days default token lifetime
	tokenOpWindow     = 1 * time.Minute
	tokenOpMaxActions = 20
)

type tokenRateEntry struct {
	count       int
	firstAction time.Time
}

var tokenRateLimiter = struct {
	sync.Mutex
	entries map[string]*tokenRateEntry
}{entries: map[string]*tokenRateEntry{}}

type tokenCreateRequest struct {
	Name      string          `json:"name"`
	UserID    *int64          `json:"user_id"`
	ExpiresAt *string         `json:"expires_at"`
	Scopes    json.RawMessage `json:"scopes"`
	IssuedFor json.RawMessage `json:"issued_for"`
}

type tokenRevokeRequest struct {
	ID int64 `json:"id"`
}

func (h *Handlers) Tokens(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listTokens(w, r)
	case http.MethodPost:
		h.createToken(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) TokenRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	rateKey := tokenRateKey(r)
	if isTokenRateLimited(rateKey) {
		writeError(w, http.StatusTooManyRequests, "too many token operations, please retry shortly")
		return
	}
	var body tokenRevokeRequest
	if err := readJSON(w, r, &body); err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ID <= 0 {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusBadRequest, "token id is required")
		return
	}
	if err := h.store.RevokeToken(body.ID); err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordTokenRateAction(rateKey)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handlers) listTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListTokens()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		results = append(results, map[string]any{
			"id":           token.ID,
			"name":         token.Name,
			"created_at":   token.CreatedAt,
			"last_used_at": token.LastUsedAt,
			"expires_at":   token.ExpiresAt,
			"scopes":       token.Scopes,
			"disabled":     token.Disabled,
			"revoked_at":   token.RevokedAt,
			"issued_for":   token.IssuedFor,
			"user_id":      token.UserID,
			"role":         token.Role,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": results})
}

func (h *Handlers) createToken(w http.ResponseWriter, r *http.Request) {
	rateKey := tokenRateKey(r)
	if isTokenRateLimited(rateKey) {
		writeError(w, http.StatusTooManyRequests, "too many token operations, please retry shortly")
		return
	}

	var body tokenCreateRequest
	if err := readJSON(w, r, &body); err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var userID *int64
	var role *string
	if body.UserID != nil {
		user, err := h.store.GetUserByIDFull(*body.UserID)
		if err != nil {
			recordTokenRateAction(rateKey)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if user == nil {
			recordTokenRateAction(rateKey)
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		userID = &user.ID
		roleValue := user.Role
		if roleValue == "" {
			roleValue = "user"
		}
		role = &roleValue
	} else if currentUserID, ok := readUserIDFromContext(r); ok {
		userID = &currentUserID
		if currentRole, ok := readUserRoleFromContext(r); ok {
			roleValue := currentRole
			role = &roleValue
		}
	}

	secret, err := generateTokenSecret()
	if err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	tokenHash, fingerprint, err := hashToken(secret)
	if err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusInternalServerError, "failed to hash token")
		return
	}

	scopes := normalizeTokenField(body.Scopes)
	issuedFor := normalizeTokenField(body.IssuedFor)

	// Apply default TTL if not specified
	expiresAt := body.ExpiresAt
	if expiresAt == nil {
		// Default to 90 days from now
		defaultExpiry := time.Now().Add(defaultTokenTTL).Format(time.RFC3339)
		expiresAt = &defaultExpiry
	}

	record := store.TokenRecord{
		Name:             name,
		TokenHash:        tokenHash,
		TokenFingerprint: fingerprint,
		UserID:           userID,
		Role:             role,
		ExpiresAt:        expiresAt,
		Scopes:           scopes,
		IssuedFor:        issuedFor,
		Disabled:         0,
	}

	created, err := h.store.CreateToken(record)
	if err != nil {
		recordTokenRateAction(rateKey)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recordTokenRateAction(rateKey)

	writeJSON(w, http.StatusOK, map[string]any{
		"token": map[string]any{
			"id":        created.ID,
			"name":      created.Name,
			"secret":    secret,
			"createdAt": created.CreatedAt,
			"expiresAt": created.ExpiresAt,
		},
	})
}

func tokenRateKey(r *http.Request) string {
	if userID, ok := readUserIDFromContext(r); ok {
		return fmt.Sprintf("uid:%d", userID)
	}
	return "ip:" + clientIP(r)
}

func isTokenRateLimited(key string) bool {
	tokenRateLimiter.Lock()
	defer tokenRateLimiter.Unlock()
	entry, ok := tokenRateLimiter.entries[key]
	if !ok {
		return false
	}
	if time.Since(entry.firstAction) > tokenOpWindow {
		delete(tokenRateLimiter.entries, key)
		return false
	}
	return entry.count >= tokenOpMaxActions
}

func recordTokenRateAction(key string) {
	tokenRateLimiter.Lock()
	defer tokenRateLimiter.Unlock()
	now := time.Now()
	entry, ok := tokenRateLimiter.entries[key]
	if !ok || now.Sub(entry.firstAction) > tokenOpWindow {
		tokenRateLimiter.entries[key] = &tokenRateEntry{count: 1, firstAction: now}
		return
	}
	entry.count++
}

func normalizeTokenField(value json.RawMessage) *string {
	if len(value) == 0 {
		return nil
	}
	var normalized string
	if json.Valid(value) {
		normalized = strings.TrimSpace(string(value))
	} else {
		normalized = strings.TrimSpace(string(value))
	}
	if normalized == "" || normalized == "null" {
		return nil
	}
	return &normalized
}

func (h *Handlers) authenticateBearer(ctx context.Context, token string) (*store.TokenRecord, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	fingerprint := tokenFingerprint(token)
	record, err := h.store.GetTokenByFingerprint(fingerprint)
	if err != nil || record == nil {
		return nil, nil
	}
	if record.Disabled == 1 || record.RevokedAt != nil {
		return nil, nil
	}
	if record.ExpiresAt != nil {
		if expiry, err := time.Parse(time.RFC3339, *record.ExpiresAt); err == nil {
			if time.Now().After(expiry) {
				return nil, nil
			}
		}
	}
	if ok := verifyToken(token, record.TokenHash); !ok {
		return nil, nil
	}

	_ = h.store.UpdateTokenLastUsed(record.ID)
	return record, nil
}

func EnsureBootstrapToken(storeHandle *store.SQLiteStore, secret string) error {
	if secret == "" {
		return nil
	}
	fingerprint := tokenFingerprint(secret)
	existing, err := storeHandle.GetTokenByFingerprint(fingerprint)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	user, err := storeHandle.GetUserByRole("super_admin")
	if err != nil {
		return err
	}
	if user == nil {
		user, err = storeHandle.GetUserByRole("admin")
		if err != nil {
			return err
		}
	}

	tokenHash, _, err := hashToken(secret)
	if err != nil {
		return err
	}

	name := "bootstrap"
	role := "super_admin"
	var userID *int64
	if user != nil {
		userID = &user.ID
		role = user.Role
		if role == "" {
			role = "super_admin"
		}
	}
	roleValue := role
	issuedFor := `{"source":"env"}`

	_, err = storeHandle.CreateToken(store.TokenRecord{
		Name:             name,
		TokenHash:        tokenHash,
		TokenFingerprint: fingerprint,
		UserID:           userID,
		Role:             &roleValue,
		IssuedFor:        &issuedFor,
		Disabled:         0,
	})
	return err
}

func generateTokenSecret() (string, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(seed), nil
}

func tokenFingerprint(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func hashToken(secret string) (string, string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}
	hash := argon2.IDKey([]byte(secret), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, tokenFingerprint(secret), nil
}

func verifyToken(secret string, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	calculated := argon2.IDKey([]byte(secret), salt, iterations, memory, parallelism, uint32(len(hash)))
	return subtleCompare(hash, calculated)
}

func subtleCompare(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func parseBearerToken(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", nil
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header")
	}
	return strings.TrimSpace(parts[1]), nil
}
