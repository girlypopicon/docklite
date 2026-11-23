package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

// Where a finished backup goes. "local" is a folder on this server (what DockLite always did). The others copy the
// package somewhere else after it is made and checked:
//
//	s3      any S3-compatible bucket: Cloudflare R2, Backblaze B2, Amazon S3, Wasabi, MinIO, Oracle
//	sftp    any server you can SSH into, which includes a second DockLite server
//	server  a DockLite server connected on the Servers page (it keeps the file in its own backups folder)
//
// Every destination can also lock its backups with an encryption key (see crypt.go). A backup on a remote destination
// is recorded with a reference like s3://bucket/key instead of a file path.
type DestinationConfig struct {
	Path string `json:"path,omitempty"` // local: the folder

	Encrypt   bool   `json:"encrypt,omitempty"`   // lock every backup sent here
	Recipient string `json:"recipient,omitempty"` // the public key to lock them with (age1...)

	// s3
	Endpoint  string `json:"endpoint,omitempty"` // https://<account>.r2.cloudflarestorage.com; empty means Amazon S3
	Region    string `json:"region,omitempty"`   // "auto" for R2
	Bucket    string `json:"bucket,omitempty"`
	Prefix    string `json:"prefix,omitempty"` // a folder inside the bucket
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`

	// sftp
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	User       string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	HostKey    string `json:"host_key,omitempty"` // SHA256:... of the server's key, pinned so nobody can stand in for it
	Dir        string `json:"dir,omitempty"`      // the folder on that server

	// server
	ServerID int64 `json:"server_id,omitempty"`
}

// secretFields are never sent back to the browser. A save that leaves one blank keeps what is stored.
var secretFields = []string{"secret_key", "password", "private_key"}

// ParseDestConfig reads a destination's saved settings. Old rows hold only a path.
func ParseDestConfig(raw string) DestinationConfig {
	var c DestinationConfig
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	return c
}

// RedactConfig hides the secrets of a saved config for the browser, saying which ones are set.
func RedactConfig(raw string) json.RawMessage {
	m := map[string]any{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return json.RawMessage(`{}`)
	}
	for _, k := range secretFields {
		if v, ok := m[k].(string); ok && v != "" {
			m[k+"_set"] = true
		}
		delete(m, k)
	}
	b, _ := json.Marshal(m)
	return b
}

// MergeConfig is a saved config updated with what the browser sent: a blank secret keeps the stored one.
func MergeConfig(old, next string) string {
	om, nm := map[string]any{}, map[string]any{}
	_ = json.Unmarshal([]byte(old), &om)
	if json.Unmarshal([]byte(next), &nm) != nil {
		return next
	}
	for _, k := range secretFields {
		if v, _ := nm[k].(string); v == "" {
			if ov, ok := om[k]; ok {
				nm[k] = ov
			}
		}
		delete(nm, k+"_set")
	}
	b, _ := json.Marshal(nm)
	return string(b)
}

// Validate says what is missing or wrong in a destination before it is saved.
func (c DestinationConfig) Validate(typ string) error {
	if c.Encrypt {
		if _, err := ParseRecipient(c.Recipient); err != nil {
			return fmt.Errorf("encryption is on but there is no valid backup key: %w", err)
		}
	}
	switch typ {
	case "local":
		return nil
	case "s3":
		if strings.TrimSpace(c.Bucket) == "" {
			return fmt.Errorf("the bucket name is required")
		}
		if c.AccessKey == "" || c.SecretKey == "" {
			return fmt.Errorf("the access key and secret key are required")
		}
		if c.Endpoint == "" && c.Region == "" {
			return fmt.Errorf("give a region (for example us-east-1), or the endpoint of the service you use")
		}
		if c.Endpoint != "" {
			return checkEndpoint(c.Endpoint)
		}
	case "sftp":
		if strings.TrimSpace(c.Host) == "" || strings.TrimSpace(c.User) == "" {
			return fmt.Errorf("the server's address and the user name are required")
		}
		if c.Password == "" && c.PrivateKey == "" {
			return fmt.Errorf("give a password or a private key")
		}
		if c.Port < 0 || c.Port > 65535 {
			return fmt.Errorf("that isn't a port")
		}
	case "server":
		if c.ServerID <= 0 {
			return fmt.Errorf("choose a connected server")
		}
	default:
		return fmt.Errorf("unknown destination type %q (use local, s3, sftp or server)", typ)
	}
	return nil
}

// checkEndpoint accepts http(s) addresses only and refuses link-local ones (cloud metadata services).
func checkEndpoint(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("the endpoint must look like https://<account>.r2.cloudflarestorage.com")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return fmt.Errorf("that address can't be a storage service")
	}
	return nil
}

// Remote is somewhere backups can be sent. name is the path inside the destination ("packages/site-….litpkg.age").
type Remote interface {
	// Put copies a file there and checks it arrived whole. The reference it returns is what is saved with the backup.
	Put(ctx context.Context, localPath, name string) (ref string, err error)
	// Get copies a file back, by the reference Put gave.
	Get(ctx context.Context, ref, localPath string) error
	Delete(ctx context.Context, ref string) error
	// Test checks the settings work: it connects, writes a tiny file and removes it. The text says what it found.
	Test(ctx context.Context) (string, error)
}

// IsRemoteRef says whether a saved backup path points at a remote destination instead of a file on this server.
func IsRemoteRef(path string) bool { return strings.Contains(path, "://") }

// OpenRemote is the Remote for a destination. Local destinations have none: their files are plain files.
func OpenRemote(st *store.SQLiteStore, dest store.BackupDestination) (Remote, error) {
	cfg := ParseDestConfig(dest.Config)
	switch dest.Type {
	case "s3":
		return newS3(cfg), nil
	case "sftp":
		return newSFTP(cfg), nil
	case "server":
		srv, err := st.GetRemoteServer(cfg.ServerID)
		if err != nil {
			return nil, err
		}
		if srv == nil {
			return nil, fmt.Errorf("the server this destination sends to is no longer connected")
		}
		return newHubRemote(*srv), nil
	}
	return nil, fmt.Errorf("%s destinations are plain folders", dest.Type)
}

// retry runs fn up to three times, for the network hiccups a long upload will meet. Cancelling stops it at once.
func retry(ctx context.Context, fn func() error) error {
	var err error
	for i, wait := range []time.Duration{0, 2 * time.Second, 6 * time.Second} {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if err = fn(); err == nil || ctx.Err() != nil {
			return err
		}
	}
	return err
}
