package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "a.litpkg")
	want := strings.Repeat("site files and a database dump\n", 5000)
	if err := os.WriteFile(plain, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	rcpt, ident, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	enc := filepath.Join(dir, "a.litpkg.age")
	size, sum, err := EncryptFile(context.Background(), plain, enc, rcpt)
	if err != nil || size == 0 || len(sum) != 64 {
		t.Fatalf("encrypt: %v size=%d sum=%q", err, size, sum)
	}
	raw, _ := os.ReadFile(enc)
	if strings.Contains(string(raw), "database dump") {
		t.Fatal("the encrypted file contains the plain text")
	}
	if st, _ := os.Stat(enc); st.Size() != size {
		t.Fatalf("reported size %d, file is %d", size, st.Size())
	}
	out := filepath.Join(dir, "back.litpkg")
	if err := DecryptFile(context.Background(), enc, out, ident); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != want {
		t.Fatal("decrypted file differs")
	}
	if r, err := RecipientOf(ident); err != nil || r != rcpt {
		t.Fatalf("RecipientOf: %q %v", r, err)
	}
}

func TestDecryptRejectsWrongKeyAndDamage(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "a")
	_ = os.WriteFile(plain, []byte(strings.Repeat("x", 200000)), 0o600)
	rcpt, right, _ := GenerateKey()
	_, other, _ := GenerateKey()
	enc := filepath.Join(dir, "a.age")
	if _, _, err := EncryptFile(context.Background(), plain, enc, rcpt); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := DecryptFile(context.Background(), enc, out, other); err == nil {
		t.Fatal("the wrong key opened the backup")
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("a failed decrypt left a file")
	}
	// cut the file short: the tail check must fail, not hand back half a backup
	raw, _ := os.ReadFile(enc)
	_ = os.WriteFile(enc, raw[:len(raw)-100], 0o600)
	if err := DecryptFile(context.Background(), enc, out, right); err == nil {
		t.Fatal("a cut-short file was accepted")
	}
}

func TestBadKeysAreRefused(t *testing.T) {
	if _, err := ParseRecipient("not a key"); err == nil {
		t.Fatal("accepted a bad recipient")
	}
	if _, err := RecipientOf("nope"); err == nil {
		t.Fatal("accepted a bad identity")
	}
}
