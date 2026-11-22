package docker

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSocket(t *testing.T) {
	if got := CheckSocket("tcp://1.2.3.4:2375"); got != nil {
		t.Fatalf("tcp host should not be checked: %v", got)
	}
	dir := t.TempDir()
	missing := "unix://" + filepath.Join(dir, "nope.sock")
	if got := CheckSocket(missing); len(got) != 1 {
		t.Fatalf("missing socket should give one problem: %v", got)
	}
	file := filepath.Join(dir, "file")
	os.WriteFile(file, []byte("x"), 0o600)
	if got := CheckSocket("unix://" + file); len(got) != 1 {
		t.Fatalf("a plain file is not a socket: %v", got)
	}
	sock := filepath.Join(dir, "d.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("cannot make a unix socket here")
	}
	defer l.Close()
	os.Chmod(sock, 0o660)
	if got := CheckSocket("unix://" + sock); len(got) != 0 {
		t.Fatalf("0660 socket should be fine: %v", got)
	}
	os.Chmod(sock, 0o666)
	if got := CheckSocket("unix://" + sock); len(got) != 1 {
		t.Fatalf("world-writable socket should warn: %v", got)
	}
}
