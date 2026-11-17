package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/totp"

	qrcode "github.com/skip2/go-qrcode"
)

// Two-factor login: after the password, a six-digit code from an authenticator app (Google Authenticator, 1Password,
// Aegis...). Nothing leaves the server: the app and DockLite share a secret and each works out the code on its own.
// Ten one-time recovery codes are shown once when it is turned on, for a lost phone. An admin can reset it for someone
// they manage, and `docklite users 2fa-reset` does the same from the server's terminal.

const recoveryCodeCount = 10

// Lowercase letters and digits without the easily confused ones (0/o, 1/l/i), in two groups of five: "k7mxp-3qhwz".
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

func newRecoveryCodes() ([]string, []string, error) {
	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([]string, 0, recoveryCodeCount)
	buf := make([]byte, 10)
	for i := 0; i < recoveryCodeCount; i++ {
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for j, c := range buf {
			if j == 5 {
				b.WriteByte('-')
			}
			b.WriteByte(recoveryAlphabet[int(c)%len(recoveryAlphabet)])
		}
		codes = append(codes, b.String())
		hashes = append(hashes, hashRecoveryCode(b.String()))
	}
	return codes, hashes, nil
}

func hashRecoveryCode(code string) string {
	norm := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// twoFactorIssuer is what the authenticator app shows above the code: "DockLite (panel.example.com)" so two servers
// don't look the same in the app.
func twoFactorIssuer(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if i := strings.IndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	if host == "" || host == "127.0.0.1" || host == "localhost" {
		return "DockLite"
	}
	return "DockLite (" + host + ")"
}

// twoFactorLogoURL is where an authenticator app can fetch the DockLite logo (apps such as Aegis, 2FAS and Ente Auth show
// it beside the code; Google Authenticator ignores it). Empty when the server has no real address to point at.
func twoFactorLogoURL(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	h := host
	if i := strings.IndexByte(h, ':'); i > 0 {
		h = h[:i]
	}
	if h == "" || h == "127.0.0.1" || h == "localhost" {
		return ""
	}
	scheme := "http"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + host + "/docklite-logo.png"
}

// verifySecondFactor checks a six-digit code or a recovery code for a user who has two-factor login on. A good code is
// marked used; a recovery code is used up. It returns false for anything else.
func (h *Handlers) verifySecondFactor(tf *store.TwoFactor, code string) (bool, string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return false, ""
	}
	digits := strings.ReplaceAll(code, " ", "")
	if len(digits) == totp.Digits && strings.Trim(digits, "0123456789") == "" {
		step, ok := totp.Check(tf.Secret, digits, time.Now(), tf.LastStep)
		if !ok {
			return false, ""
		}
		_ = h.store.MarkTwoFactorStep(tf.UserID, step)
		return true, "code"
	}
	want := hashRecoveryCode(code)
	for i, hsh := range tf.RecoveryHashes {
		if subtle.ConstantTimeCompare([]byte(hsh), []byte(want)) == 1 {
			rest := append(append([]string{}, tf.RecoveryHashes[:i]...), tf.RecoveryHashes[i+1:]...)
			_ = h.store.SetRecoveryHashes(tf.UserID, rest)
			return true, "recovery"
		}
	}
	return false, ""
}

// TwoFactor serves /api/auth/2fa (GET: is it on?) and its actions under /api/auth/2fa/...
func (h *Handlers) TwoFactor(w http.ResponseWriter, r *http.Request) {
	userID, ok := readUserIDFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return
	}
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth/2fa"), "/")

	if action == "" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		tf, err := h.store.GetTwoFactor(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		on := tf != nil && tf.Enabled
		left := 0
		if on {
			left = len(tf.RecoveryHashes)
		}
		writeJSON(w, http.StatusOK, map[string]any{"enabled": on, "recovery_codes_left": left, "enabled_at": enabledAt(tf)})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	switch action {
	case "setup":
		h.twoFactorSetup(w, r, userID)
	case "enable":
		h.twoFactorEnable(w, r, userID)
	case "disable":
		h.twoFactorDisable(w, r, userID)
	case "recovery-codes":
		h.twoFactorNewRecoveryCodes(w, r, userID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func enabledAt(tf *store.TwoFactor) string {
	if tf == nil || !tf.Enabled {
		return ""
	}
	return tf.EnabledAt
}

// setup makes a new secret and returns it with its QR code. Nothing changes at login until enable confirms a code.
func (h *Handlers) twoFactorSetup(w http.ResponseWriter, r *http.Request, userID int64) {
	user, err := h.store.GetUserByIDFull(userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	secret, err := totp.NewSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not make a secret")
		return
	}
	if err := h.store.StartTwoFactor(userID, secret); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	link := totp.URL(secret, twoFactorIssuer(r), user.Username)
	if img := twoFactorLogoURL(r); img != "" {
		link += "&image=" + url.QueryEscape(img)
	}
	png, err := qrcode.Encode(link, qrcode.Medium, 320)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not draw the QR code")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"secret":      secret,
		"otpauth_url": link,
		"qr_png":      "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (h *Handlers) twoFactorEnable(w http.ResponseWriter, r *http.Request, userID int64) {
	var body struct {
		Code string `json:"code"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tf, err := h.store.GetTwoFactor(userID)
	if err != nil || tf == nil {
		writeError(w, http.StatusBadRequest, "start the setup first")
		return
	}
	if tf.Enabled {
		writeError(w, http.StatusConflict, "two-factor login is already on")
		return
	}
	rateKey := "2fa-enable:" + strconv.FormatInt(userID, 10)
	if isRateLimited(rateKey) {
		writeError(w, http.StatusTooManyRequests, "too many tries; wait a few minutes")
		return
	}
	step, ok := totp.Check(tf.Secret, body.Code, time.Now(), 0)
	if !ok {
		recordAttempt(rateKey)
		writeError(w, http.StatusBadRequest, "that code doesn't match; check the phone's clock and try the newest code")
		return
	}
	clearAttempts(rateKey)
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not make recovery codes")
		return
	}
	if err := h.store.EnableTwoFactor(userID, step, hashes); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "user.2fa-enable", strconv.FormatInt(userID, 10), nil)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "recovery_codes": codes})
}

// disable needs the password and a code (or a recovery code), so a stolen session alone can't turn it off.
func (h *Handlers) twoFactorDisable(w http.ResponseWriter, r *http.Request, userID int64) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tf, ok := h.confirmUserAndCode(w, r, userID, body.Password, body.Code)
	if !ok {
		return
	}
	if tf == nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	if err := h.store.DeleteTwoFactor(userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "user.2fa-disable", strconv.FormatInt(userID, 10), nil)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handlers) twoFactorNewRecoveryCodes(w http.ResponseWriter, r *http.Request, userID int64) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tf, ok := h.confirmUserAndCode(w, r, userID, body.Password, body.Code)
	if !ok {
		return
	}
	if tf == nil {
		writeError(w, http.StatusBadRequest, "two-factor login is off")
		return
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not make recovery codes")
		return
	}
	if err := h.store.SetRecoveryHashes(userID, hashes); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "user.2fa-recovery-codes", strconv.FormatInt(userID, 10), nil)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "recovery_codes": codes})
}

// confirmUserAndCode checks the user's password and, when two-factor login is on, a code. It writes the error itself
// and returns ok=false when either is wrong. tf is nil when two-factor login is off.
func (h *Handlers) confirmUserAndCode(w http.ResponseWriter, r *http.Request, userID int64, password, code string) (*store.TwoFactor, bool) {
	rateKey := "2fa-confirm:" + strconv.FormatInt(userID, 10)
	if isRateLimited(rateKey) {
		writeError(w, http.StatusTooManyRequests, "too many tries; wait a few minutes")
		return nil, false
	}
	user, err := h.store.GetUserByIDFull(userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return nil, false
	}
	if err := h.store.VerifyPassword(user, password); err != nil {
		recordAttempt(rateKey)
		writeError(w, http.StatusUnauthorized, "wrong password")
		return nil, false
	}
	tf, err := h.store.GetTwoFactor(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if tf == nil || !tf.Enabled {
		clearAttempts(rateKey)
		return nil, true
	}
	if ok, _ := h.verifySecondFactor(tf, code); !ok {
		recordAttempt(rateKey)
		writeError(w, http.StatusUnauthorized, "wrong code")
		return nil, false
	}
	clearAttempts(rateKey)
	return tf, true
}

// TwoFactorReset is POST /api/users/2fa-reset {"userId": n}: an admin turns off two-factor login for someone they
// manage (a lost phone with no recovery codes left). The person can turn it on again with a new phone.
func (h *Handlers) TwoFactorReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		UserID   int64  `json:"userId"`
		Username string `json:"username"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var target *store.UserRecord
	var err error
	if body.UserID > 0 {
		target, err = h.store.GetUserByIDFull(body.UserID)
	} else if body.Username != "" {
		target, err = h.store.GetUserByUsername(body.Username)
	}
	if err != nil || target == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if !canManageUser(r, target) {
		writeError(w, http.StatusForbidden, "you can't manage this user")
		return
	}
	if err := h.store.DeleteTwoFactor(target.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "user.2fa-reset", target.Username, map[string]any{"userId": target.ID})
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "username": target.Username})
}
