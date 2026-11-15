package totp

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 column. The RFC prints eight digits; an authenticator shows the last six.
func TestRFC6238Vectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		got, err := CodeAt(secret, tc.unix/Period)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("t=%d: got %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestCheckAllowsOneStepAndRefusesReuse(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	cur := Step(now)
	prev, _ := CodeAt(secret, cur-1)
	next, _ := CodeAt(secret, cur+1)
	old, _ := CodeAt(secret, cur-2)

	if step, ok := Check(secret, prev, now, 0); !ok || step != cur-1 {
		t.Fatalf("previous step should pass, got %d %v", step, ok)
	}
	if _, ok := Check(secret, next, now, 0); !ok {
		t.Fatal("next step should pass")
	}
	if _, ok := Check(secret, old, now, 0); ok {
		t.Fatal("two steps back should fail")
	}
	// Once a step has been used, its code (and older ones) no longer work.
	if _, ok := Check(secret, prev, now, cur-1); ok {
		t.Fatal("a used code must not work twice")
	}
	if _, ok := Check(secret, "12345", now, 0); ok {
		t.Fatal("five digits should fail")
	}
	spaced, _ := CodeAt(secret, cur)
	if _, ok := Check(secret, spaced[:3]+" "+spaced[3:], now, 0); !ok {
		t.Fatal("a code typed with a space in the middle should pass")
	}
}

func TestURL(t *testing.T) {
	u := URL("ABC", "DockLite (example.com)", "stella")
	if !strings.HasPrefix(u, "otpauth://totp/DockLite%20%28example.com%29:stella?") || !strings.Contains(u, "secret=ABC") {
		t.Fatalf("unexpected url %s", u)
	}
}
