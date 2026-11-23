package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
)

// Encrypted backups use age (https://age-encryption.org), a small standard format, so a backup can be opened without
// DockLite: `age -d -i key.txt -o backup.litpkg backup.litpkg.age`.
//
// The server keeps only the public half of the key (the "recipient"). It can lock backups but not unlock them, so a
// stolen server or a stolen bucket doesn't give anyone the old backups. The private half (the "identity") is shown once
// when the key is made, and is only ever needed to restore, when the person pastes it in.
const EncryptedExt = ".age"

// IsEncrypted says whether a backup file is an age file, by its name.
func IsEncrypted(path string) bool { return strings.HasSuffix(strings.ToLower(path), EncryptedExt) }

// GenerateKey makes a new key pair.
func GenerateKey() (recipient, identity string, err error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", err
	}
	return id.Recipient().String(), id.String(), nil
}

// ParseRecipient checks a public key and returns it.
func ParseRecipient(s string) (age.Recipient, error) {
	s = strings.TrimSpace(s)
	r, err := age.ParseX25519Recipient(s)
	if err != nil {
		return nil, fmt.Errorf("that isn't a backup key (it should start with age1): %w", err)
	}
	return r, nil
}

// RecipientOf gives the public key that goes with a private one, so a pasted key can be checked against a destination.
func RecipientOf(identity string) (string, error) {
	ids, err := parseIdentities(identity)
	if err != nil {
		return "", err
	}
	for _, id := range ids {
		if x, ok := id.(*age.X25519Identity); ok {
			return x.Recipient().String(), nil
		}
	}
	return "", fmt.Errorf("that isn't a backup key")
}

func parseIdentities(identity string) ([]age.Identity, error) {
	ids, err := age.ParseIdentities(strings.NewReader(identity))
	if err != nil || len(ids) == 0 {
		return nil, fmt.Errorf("that isn't a backup key (it should start with AGE-SECRET-KEY-)")
	}
	return ids, nil
}

// EncryptFile writes an encrypted copy of src at dst and returns the copy's size and sha256. dst is removed on failure.
func EncryptFile(ctx context.Context, src, dst, recipient string) (size int64, sum string, err error) {
	rcpt, err := ParseRecipient(recipient)
	if err != nil {
		return 0, "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return 0, "", err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		out.Close()
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	h := sha256.New()
	cw := &teeCount{w: io.MultiWriter(out, h)}
	w, err := age.Encrypt(cw, rcpt)
	if err != nil {
		return 0, "", err
	}
	if _, err = io.Copy(w, &ctxReader{ctx: ctx, r: in}); err != nil {
		return 0, "", err
	}
	if err = w.Close(); err != nil {
		return 0, "", err
	}
	if err = out.Sync(); err != nil {
		return 0, "", err
	}
	return cw.n, hex.EncodeToString(h.Sum(nil)), nil
}

// DecryptFile opens an age file with a private key and writes the plain file at dst. A wrong key, or a file that was
// cut short or changed, is an error and leaves nothing at dst.
func DecryptFile(ctx context.Context, src, dst, identity string) (err error) {
	ids, err := parseIdentities(identity)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	r, err := age.Decrypt(in, ids...)
	if err != nil {
		return fmt.Errorf("that key doesn't open this backup: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	if _, err = io.Copy(out, &ctxReader{ctx: ctx, r: r}); err != nil {
		return fmt.Errorf("the backup is damaged or incomplete: %w", err)
	}
	return out.Sync()
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// teeCount passes writes on and counts the bytes.
type teeCount struct {
	w io.Writer
	n int64
}

func (t *teeCount) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	t.n += int64(n)
	return n, err
}
