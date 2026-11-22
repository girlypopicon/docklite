package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const delegationCookieName = "docklite_delegation"

type delegationClaims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	Admin  bool   `json:"admin"`
	Expiry int64  `json:"exp"`
	// PV ties the session to the password it was made with: changing the password ends every older session.
	PV string `json:"pv,omitempty"`
}

type contextKey string

const (
	ctxUserIDKey   contextKey = "dockliteUserID"
	ctxUserRoleKey contextKey = "dockliteUserRole"
	ctxSessionPV   contextKey = "dockliteSessionPV"
)

// passwordVersion is a short fingerprint of a password hash (not reversible to it).
func passwordVersion(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return base64.RawURLEncoding.EncodeToString(sum[:9])
}

func withDelegationContext(r *http.Request, secret string) (*http.Request, bool) {
	cookie, err := r.Cookie(delegationCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}

	claims, ok := parseDelegationToken(cookie.Value, secret)
	if !ok {
		return nil, false
	}

	ctx := context.WithValue(r.Context(), ctxUserIDKey, claims.UserID)
	ctx = context.WithValue(ctx, ctxUserRoleKey, claims.Role)
	ctx = context.WithValue(ctx, ctxSessionPV, claims.PV)
	return r.WithContext(ctx), true
}

func parseDelegationToken(token string, secret string) (*delegationClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, false
	}

	payloadPart := parts[0]
	signaturePart := parts[1]

	signature, err := base64.RawURLEncoding.DecodeString(signaturePart)
	if err != nil {
		return nil, false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadPart))
	expected := mac.Sum(nil)
	if !hmac.Equal(signature, expected) {
		return nil, false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, false
	}

	var claims delegationClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, false
	}

	if claims.Expiry != 0 && time.Now().Unix() > claims.Expiry {
		return nil, false
	}

	return &claims, true
}

func readUserIDFromContext(r *http.Request) (int64, bool) {
	value := r.Context().Value(ctxUserIDKey)
	if value == nil {
		return 0, false
	}
	userID, ok := value.(int64)
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func readUserRoleFromContext(r *http.Request) (string, bool) {
	value := r.Context().Value(ctxUserRoleKey)
	if value == nil {
		return "", false
	}
	role, ok := value.(string)
	if !ok || role == "" {
		return "", false
	}
	return role, true
}
