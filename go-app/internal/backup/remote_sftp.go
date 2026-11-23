package backup

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// An SFTP destination is any server you can SSH into: a second DockLite server, a storage box, a home machine. The
// server's key is pinned the first time you save it, so a later connection to something that isn't that server is refused.
type sftpRemote struct{ cfg DestinationConfig }

func newSFTP(cfg DestinationConfig) *sftpRemote { return &sftpRemote{cfg: cfg} }

func (s *sftpRemote) addr() string {
	p := s.cfg.Port
	if p == 0 {
		p = 22
	}
	return net.JoinHostPort(strings.TrimSpace(s.cfg.Host), strconv.Itoa(p))
}

func (s *sftpRemote) auth() ([]ssh.AuthMethod, error) {
	var m []ssh.AuthMethod
	if s.cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("the private key can't be used (keys with a passphrase aren't supported; make one without): %w", err)
		}
		m = append(m, ssh.PublicKeys(signer))
	}
	if s.cfg.Password != "" {
		m = append(m, ssh.Password(s.cfg.Password))
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("give a password or a private key")
	}
	return m, nil
}

func (s *sftpRemote) connect(ctx context.Context, pin func(ssh.PublicKey) error) (*ssh.Client, *sftp.Client, error) {
	auth, err := s.auth()
	if err != nil {
		return nil, nil, err
	}
	conf := &ssh.ClientConfig{
		User: s.cfg.User, Auth: auth, Timeout: 20 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error { return pin(key) },
	}
	d := net.Dialer{Timeout: 20 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", s.addr())
	if err != nil {
		return nil, nil, fmt.Errorf("can't reach %s: %w", s.addr(), err)
	}
	sc, chans, reqs, err := ssh.NewClientConn(conn, s.addr(), conf)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	client := ssh.NewClient(sc, chans, reqs)
	ft, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("the server doesn't offer file transfer (SFTP): %w", err)
	}
	return client, ft, nil
}

// pinned accepts only the key whose fingerprint was saved.
func (s *sftpRemote) pinned(key ssh.PublicKey) error {
	if s.cfg.HostKey == "" {
		return fmt.Errorf("this server's key hasn't been saved yet; test the destination first")
	}
	if got := ssh.FingerprintSHA256(key); got != s.cfg.HostKey {
		return fmt.Errorf("the server's key is %s, not the %s that was saved. If the server was reinstalled, save the destination again; otherwise something is pretending to be it", got, s.cfg.HostKey)
	}
	return nil
}

// ScanHostKey connects only to read the server's key (and stops there, so no password is sent).
func ScanHostKey(ctx context.Context, host string, port int) (string, error) {
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port))
	var fp string
	conf := &ssh.ClientConfig{
		User: "docklite-probe", Timeout: 15 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			fp = ssh.FingerprintSHA256(key)
			return fmt.Errorf("key read")
		},
	}
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("can't reach %s: %w", addr, err)
	}
	defer conn.Close()
	_, _, _, _ = ssh.NewClientConn(conn, addr, conf)
	if fp == "" {
		return "", fmt.Errorf("%s didn't answer like an SSH server", addr)
	}
	return fp, nil
}

func (s *sftpRemote) full(name string) string {
	dir := s.cfg.Dir
	if dir == "" {
		dir = "docklite-backups"
	}
	return path.Join(dir, path.Clean("/"+name))
}

// A reference keeps absolute folders as they are and marks folders relative to the login folder with "/~/".
func (s *sftpRemote) ref(p string) string {
	if path.IsAbs(p) {
		return "sftp://" + s.addr() + p
	}
	return "sftp://" + s.addr() + "/~/" + p
}

// pathFromRef is the file a reference points at, only if it is on this destination's server and inside its folder.
func (s *sftpRemote) pathFromRef(ref string) (string, error) {
	rest, ok := strings.CutPrefix(ref, "sftp://"+s.addr())
	if !ok || !strings.HasPrefix(rest, "/") {
		return "", fmt.Errorf("%s is not on %s", ref, s.addr())
	}
	p := path.Clean(rest)
	if rel, ok := strings.CutPrefix(rest, "/~/"); ok {
		p = path.Clean(rel)
	}
	if !strings.HasPrefix(p, s.full("")+"/") {
		return "", fmt.Errorf("%s is outside the backup folder", ref)
	}
	return p, nil
}

func (s *sftpRemote) Put(ctx context.Context, localPath, name string) (string, error) {
	cl, ft, err := s.connect(ctx, s.pinned)
	if err != nil {
		return "", err
	}
	defer cl.Close()
	defer ft.Close()
	dst := s.full(name)
	if err := ft.MkdirAll(path.Dir(dst)); err != nil {
		return "", fmt.Errorf("can't make the folder %s there: %w", path.Dir(dst), err)
	}
	in, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer in.Close()
	st, _ := in.Stat()
	tmp := dst + ".partial"
	out, err := ft.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(out, &ctxReader{ctx: ctx, r: in})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && st != nil && n != st.Size() {
		err = fmt.Errorf("only %d of %d bytes arrived", n, st.Size())
	}
	if err == nil {
		if rs, e := ft.Stat(tmp); e != nil || (st != nil && rs.Size() != st.Size()) {
			err = fmt.Errorf("the copy on the server isn't the same size")
		}
	}
	if err != nil {
		_ = ft.Remove(tmp)
		return "", err
	}
	if err := ft.PosixRename(tmp, dst); err != nil {
		_ = ft.Remove(dst)
		if err2 := ft.Rename(tmp, dst); err2 != nil {
			_ = ft.Remove(tmp)
			return "", err2
		}
	}
	return s.ref(dst), nil
}

func (s *sftpRemote) Get(ctx context.Context, ref, localPath string) error {
	p, err := s.pathFromRef(ref)
	if err != nil {
		return err
	}
	cl, ft, err := s.connect(ctx, s.pinned)
	if err != nil {
		return err
	}
	defer cl.Close()
	defer ft.Close()
	in, err := ft.Open(p)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, &ctxReader{ctx: ctx, r: in})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(localPath)
	}
	return err
}

func (s *sftpRemote) Delete(ctx context.Context, ref string) error {
	p, err := s.pathFromRef(ref)
	if err != nil {
		return err
	}
	cl, ft, err := s.connect(ctx, s.pinned)
	if err != nil {
		return err
	}
	defer cl.Close()
	defer ft.Close()
	return ft.Remove(p)
}

func (s *sftpRemote) Test(ctx context.Context) (string, error) {
	cl, ft, err := s.connect(ctx, s.pinned)
	if err != nil {
		return "", err
	}
	defer cl.Close()
	defer ft.Close()
	dir := s.full("")
	if err := ft.MkdirAll(dir); err != nil {
		return "", fmt.Errorf("can't make the folder %s there: %w", dir, err)
	}
	probe := path.Join(dir, ".docklite-test")
	f, err := ft.Create(probe)
	if err != nil {
		return "", fmt.Errorf("can't write in %s: %w", dir, err)
	}
	_, _ = f.Write([]byte("docklite"))
	_ = f.Close()
	if err := ft.Remove(probe); err != nil {
		return "", fmt.Errorf("it can write but not delete, so old backups could never be cleaned up: %w", err)
	}
	return "Connected to " + s.addr() + ": can write and delete in " + dir + ".", nil
}
