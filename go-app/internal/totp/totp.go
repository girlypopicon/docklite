// Package totp makes and checks the six-digit codes that authenticator apps show (RFC 6238, the "changes every 30
// seconds" kind). The server and the phone share a secret; each works out the code for the current 30-second step on
// its own, so nothing goes over the network. Uses SHA-1, six digits and 30 seconds, which is what every authenticator
// app expects from an otpauth:// link that doesn't say otherwise.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	Period = 30 // seconds per code
	Digits = 6
	// A code from the step before or after still counts, so a phone whose clock is a little off, or someone who types
	// slowly, still gets in. RFC 6238 recommends at most one step either way.
	skew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a random 160-bit secret in base32, the form authenticator apps take.
func NewSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// Step is the 30-second step a moment falls in.
func Step(t time.Time) int64 { return t.Unix() / Period }

// CodeAt is the code for one step.
func CodeAt(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", fmt.Errorf("invalid secret: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, n%1_000_000), nil
}

// Check says whether code is right for now, allowing one step of clock drift either way. It returns the step that
// matched, so the caller can refuse the same code twice (a code seen over someone's shoulder is useless once used).
// Steps at or before notAfter are refused for that reason; pass 0 when nothing has been used yet.
func Check(secret, code string, now time.Time, notAfter int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != Digits {
		return 0, false
	}
	cur := Step(now)
	for d := -skew; d <= skew; d++ {
		step := cur + int64(d)
		if step <= notAfter {
			continue
		}
		want, err := CodeAt(secret, step)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// URL is the otpauth:// link an authenticator app reads from the QR code. issuer is what the app shows above the code
// ("DockLite (example.com)"), account is the user name.
func URL(secret, issuer, account string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(Digits))
	q.Set("period", fmt.Sprint(Period))
	return "otpauth://totp/" + label + "?" + q.Encode()
}
