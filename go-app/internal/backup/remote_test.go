package backup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeS3 keeps objects in memory and understands single puts, multipart uploads, get and delete.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	parts   map[string]map[string][]byte // upload id -> part number -> data
	auth    []string
}

func newFakeS3() *fakeS3 {
	return &fakeS3{objects: map[string][]byte{}, parts: map[string]map[string][]byte{}}
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	body, _ := io.ReadAll(r.Body)
	key := r.URL.Path
	q := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		f.parts["u1"] = map[string][]byte{}
		_, _ = w.Write([]byte(`<InitiateMultipartUploadResult><UploadId>u1</UploadId></InitiateMultipartUploadResult>`))
	case r.Method == http.MethodPut && q.Get("uploadId") != "":
		f.parts[q.Get("uploadId")][q.Get("partNumber")] = body
		w.Header().Set("ETag", `"etag`+q.Get("partNumber")+`"`)
	case r.Method == http.MethodPost && q.Get("uploadId") != "":
		var all []byte
		for i := 1; ; i++ {
			p, ok := f.parts[q.Get("uploadId")][string(rune('0'+i))]
			if !ok {
				break
			}
			all = append(all, p...)
		}
		f.objects[key] = all
		_, _ = w.Write([]byte(`<CompleteMultipartUploadResult/>`))
	case r.Method == http.MethodDelete && q.Get("uploadId") != "":
		delete(f.parts, q.Get("uploadId"))
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut:
		f.objects[key] = body
	case r.Method == http.MethodGet:
		b, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>nope</Message></Error>`))
			return
		}
		_, _ = w.Write(b)
	case r.Method == http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func s3For(srv *httptest.Server) *s3Remote {
	return newS3(DestinationConfig{Endpoint: srv.URL, Region: "auto", Bucket: "bk", Prefix: "docklite", AccessKey: "AK", SecretKey: "SK"})
}

func TestS3PutGetDelete(t *testing.T) {
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s := s3For(srv)
	dir := t.TempDir()
	src := filepath.Join(dir, "a.litpkg.age")
	_ = os.WriteFile(src, []byte("backup bytes"), 0o600)

	ref, err := s.Put(context.Background(), src, "packages/a.litpkg.age")
	if err != nil || ref != "s3://bk/docklite/packages/a.litpkg.age" {
		t.Fatalf("put: %q %v", ref, err)
	}
	if string(fake.objects["/bk/docklite/packages/a.litpkg.age"]) != "backup bytes" {
		t.Fatalf("stored %v", fake.objects)
	}
	for _, a := range fake.auth {
		if !strings.HasPrefix(a, "AWS4-HMAC-SHA256 Credential=AK/") || strings.Contains(a, "SK") {
			t.Fatalf("bad authorization header %q", a)
		}
	}
	back := filepath.Join(dir, "back")
	if err := s.Get(context.Background(), ref, back); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(back); string(b) != "backup bytes" {
		t.Fatalf("got %q", b)
	}
	if err := s.Delete(context.Background(), ref); err != nil || len(fake.objects) != 0 {
		t.Fatalf("delete: %v %v", err, fake.objects)
	}
	if err := s.Get(context.Background(), ref, back); err == nil {
		t.Fatal("getting a deleted object worked")
	}
	if _, err := os.Stat(back + "x"); err == nil {
		t.Fatal("stray file")
	}
}

func TestS3RefersOnlyToItsOwnBucket(t *testing.T) {
	s := s3For(httptest.NewServer(newFakeS3()))
	if err := s.Delete(context.Background(), "s3://other/key"); err == nil {
		t.Fatal("deleted from a bucket this destination doesn't own")
	}
}

func TestS3MultipartUsedForBigFiles(t *testing.T) {
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s := s3For(srv)
	// 5 bytes over the single-request limit would be 64 MB of test data; shrink the limits instead
	big := strings.Repeat("0123456789", 7<<20/10*10) // about 70 MB
	src := filepath.Join(t.TempDir(), "big")
	_ = os.WriteFile(src, []byte(big), 0o600)
	if _, err := s.Put(context.Background(), src, "packages/big.age"); err != nil {
		t.Fatal(err)
	}
	if got := fake.objects["/bk/docklite/packages/big.age"]; string(got) != big {
		t.Fatalf("multipart object is %d bytes, want %d", len(got), len(big))
	}
	if len(fake.parts) != 1 {
		t.Fatalf("expected the upload to be finished, parts left: %v", len(fake.parts))
	}
}

func TestS3ErrorsAreReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<Error><Code>SignatureDoesNotMatch</Code><Message>The request signature we calculated does not match.</Message></Error>`))
	}))
	defer srv.Close()
	_, err := s3For(srv).Test(context.Background())
	if err == nil || !strings.Contains(err.Error(), "secret key") {
		t.Fatalf("error should say what to check: %v", err)
	}
}

func TestConfigSecretsAreHiddenAndKept(t *testing.T) {
	raw := `{"bucket":"bk","access_key":"AK","secret_key":"SK","encrypt":true,"recipient":"age1x"}`
	var shown map[string]any
	_ = json.Unmarshal(RedactConfig(raw), &shown)
	if _, ok := shown["secret_key"]; ok || shown["secret_key_set"] != true || shown["bucket"] != "bk" {
		t.Fatalf("redacted wrongly: %v", shown)
	}
	// the browser sends the form back without the secret: the stored one stays
	merged := ParseDestConfig(MergeConfig(raw, `{"bucket":"bk2","access_key":"AK","secret_key":"","secret_key_set":true}`))
	if merged.SecretKey != "SK" || merged.Bucket != "bk2" {
		t.Fatalf("merged wrongly: %+v", merged)
	}
	// a new secret replaces it
	merged = ParseDestConfig(MergeConfig(raw, `{"secret_key":"NEW"}`))
	if merged.SecretKey != "NEW" {
		t.Fatalf("new secret lost: %+v", merged)
	}
}

func TestConfigValidation(t *testing.T) {
	good, _, _ := GenerateKey()
	cases := []struct {
		typ string
		cfg DestinationConfig
		ok  bool
	}{
		{"local", DestinationConfig{}, true},
		{"s3", DestinationConfig{Bucket: "b", AccessKey: "a", SecretKey: "s", Region: "us-east-1"}, true},
		{"s3", DestinationConfig{Bucket: "b", AccessKey: "a", SecretKey: "s"}, false},
		{"s3", DestinationConfig{Bucket: "b", AccessKey: "a", SecretKey: "s", Endpoint: "http://169.254.169.254"}, false},
		{"s3", DestinationConfig{AccessKey: "a", SecretKey: "s", Region: "x"}, false},
		{"sftp", DestinationConfig{Host: "h", User: "u", Password: "p"}, true},
		{"sftp", DestinationConfig{Host: "h", User: "u"}, false},
		{"server", DestinationConfig{ServerID: 2}, true},
		{"server", DestinationConfig{}, false},
		{"ftp", DestinationConfig{}, false},
		{"local", DestinationConfig{Encrypt: true}, false},
		{"local", DestinationConfig{Encrypt: true, Recipient: good}, true},
	}
	for i, c := range cases {
		if err := c.cfg.Validate(c.typ); (err == nil) != c.ok {
			t.Errorf("case %d (%s): ok=%v err=%v", i, c.typ, c.ok, err)
		}
	}
}

func TestSFTPRefsStayInsideTheFolder(t *testing.T) {
	s := newSFTP(DestinationConfig{Host: "box", Port: 2222, Dir: "/srv/backups"})
	ref := s.ref(s.full("packages/a.age"))
	if ref != "sftp://box:2222/srv/backups/packages/a.age" {
		t.Fatal(ref)
	}
	if p, err := s.pathFromRef(ref); err != nil || p != "/srv/backups/packages/a.age" {
		t.Fatalf("%q %v", p, err)
	}
	for _, bad := range []string{"sftp://box:2222/etc/passwd", "sftp://box:2222/srv/backups/../../etc/passwd", "sftp://other:22/srv/backups/a"} {
		if _, err := s.pathFromRef(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	rel := newSFTP(DestinationConfig{Host: "box", Dir: "dl"})
	r2 := rel.ref(rel.full("a.age"))
	if p, err := rel.pathFromRef(r2); err != nil || p != "dl/a.age" {
		t.Fatalf("%s -> %q %v", r2, p, err)
	}
}
