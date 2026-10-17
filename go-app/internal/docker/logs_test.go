package docker

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
)

func TestReadLogStreamRemovesDockerFraming(t *testing.T) {
	var framed bytes.Buffer
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("hello from stdout\n"))
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stderr).Write([]byte("an error line\n"))
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("🚀 server running\n"))

	got, err := readLogStream(bytes.NewReader(framed.Bytes()), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello from stdout\nan error line\n🚀 server running\n" {
		t.Fatalf("framing leaked into the text: %q", got)
	}
	// A container with a terminal sends plain text, which must come through untouched.
	plain, err := readLogStream(strings.NewReader("plain tty output\n"), true)
	if err != nil || plain != "plain tty output\n" {
		t.Fatalf("tty logs must pass through as they are: %q %v", plain, err)
	}
}
