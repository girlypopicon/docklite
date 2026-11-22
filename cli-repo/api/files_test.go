package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListFilesBuildsPathsAndReadsAgentFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"path":"stella/a.com","entries":[{"name":"public","isDirectory":true,"size":0},{"name":"index.html","isDirectory":false,"size":2048}]}`))
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	res, err := c.ListFiles("stella/a.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Entries[0].Path != "stella/a.com/public" || !res.Entries[0].IsDir {
		t.Fatalf("folder entry wrong: %+v", res.Entries[0])
	}
	if res.Entries[1].Path != "stella/a.com/index.html" || res.Entries[1].Size != 2048 {
		t.Fatalf("file entry wrong: %+v", res.Entries[1])
	}
}

func TestJoinFilePathAtRoot(t *testing.T) {
	if got := joinFilePath("", "stella"); got != "stella" {
		t.Fatal(got)
	}
}
