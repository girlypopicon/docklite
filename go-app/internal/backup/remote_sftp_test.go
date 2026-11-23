package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sshServer is a tiny SFTP server on a loopback port that accepts one password, serving a folder on disk.
func sshServer(t *testing.T) (host string, port int, fingerprint string) {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	conf := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if c.User() == "backup" && string(pw) == "hunter2" {
			return nil, nil
		}
		return nil, os.ErrPermission
	}}
	conf.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(conn, conf)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for nc := range chans {
					if nc.ChannelType() != "session" {
						_ = nc.Reject(ssh.UnknownChannelType, "no")
						continue
					}
					ch, reqs, _ := nc.Accept()
					go func() {
						for r := range reqs {
							ok := r.Type == "subsystem" && len(r.Payload) > 4 && string(r.Payload[4:]) == "sftp"
							_ = r.Reply(ok, nil)
							if ok {
								srv, _ := sftp.NewServer(ch)
								_ = srv.Serve()
								ch.Close()
							}
						}
					}()
				}
			}()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, ssh.FingerprintSHA256(signer.PublicKey())
}

func TestSFTPPutGetDeleteWithPinnedKey(t *testing.T) {
	host, port, fp := sshServer(t)
	root := t.TempDir()
	dir := filepath.Join(root, "backups")
	ctx := context.Background()

	scanned, err := ScanHostKey(ctx, host, port)
	if err != nil || scanned != fp {
		t.Fatalf("scan: %q (want %q) %v", scanned, fp, err)
	}

	cfg := DestinationConfig{Host: host, Port: port, User: "backup", Password: "hunter2", Dir: dir, HostKey: fp}
	s := newSFTP(cfg)
	msg, err := s.Test(ctx)
	if err != nil || !strings.Contains(msg, strconv.Itoa(port)) {
		t.Fatalf("test: %q %v", msg, err)
	}

	src := filepath.Join(t.TempDir(), "a.litpkg.age")
	_ = os.WriteFile(src, []byte(strings.Repeat("locked ", 100000)), 0o600)
	ref, err := s.Put(ctx, src, "packages/a.litpkg.age")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "packages", "a.litpkg.age")); err != nil {
		t.Fatalf("file not there: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "packages", "*.partial")); len(left) != 0 {
		t.Fatalf("partial left behind: %v", left)
	}
	back := filepath.Join(t.TempDir(), "back")
	if err := s.Get(ctx, ref, back); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(src)
	b, _ := os.ReadFile(back)
	if string(a) != string(b) {
		t.Fatal("downloaded file differs")
	}
	if err := s.Delete(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "packages", "a.litpkg.age")); !os.IsNotExist(err) {
		t.Fatal("not deleted")
	}

	// a reference outside the folder is refused, even on the right server
	if err := s.Delete(ctx, "sftp://"+s.addr()+filepath.Join(root, "secret")); err == nil {
		t.Fatal("deleted outside the backup folder")
	}

	// the wrong key (something pretending to be the server) is refused before a password is sent
	cfg.HostKey = "SHA256:definitelynottherightkey"
	if _, err := newSFTP(cfg).Test(ctx); err == nil || !strings.Contains(err.Error(), "pretending") {
		t.Fatalf("a different key was accepted: %v", err)
	}
	// and an unpinned destination won't connect at all
	cfg.HostKey = ""
	if _, err := newSFTP(cfg).Test(ctx); err == nil {
		t.Fatal("connected without a pinned key")
	}
	// a wrong password is an error, not a hang
	cfg.HostKey, cfg.Password = fp, "wrong"
	if _, err := newSFTP(cfg).Test(ctx); err == nil {
		t.Fatal("wrong password accepted")
	}
}
