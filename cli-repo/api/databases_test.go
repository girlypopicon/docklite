package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDatabaseCatalogAddsSQLiteFilesAndSites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/databases":
			w.Write([]byte(`{"databases":[{"id":"c1","name":"shop","type":"postgres","port":5432,"status":"running"}]}`))
		case "/api/databases/stats":
			w.Write([]byte(`{"databases":[
{"name":"shop","type":"postgres","container_id":"c1","size":2048,"used_by":"shop.example.com"},
{"name":"board","type":"sqlite","container_id":"site1","size":4096,"used_by":"stellagauthier.com","db_path":"/app/board.db"}]}`))
		}
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	res, err := c.GetDatabaseCatalog()
	if err != nil || len(res.Databases) != 2 {
		t.Fatalf("got %+v %v", res, err)
	}
	if res.Databases[0].UsedBy != "shop.example.com" || res.Databases[0].Size != 2048 {
		t.Fatalf("docker db not enriched: %+v", res.Databases[0])
	}
	sq := res.Databases[1]
	if !sq.IsFile || sq.Type != "sqlite" || sq.UsedBy != "stellagauthier.com" || sq.DBPath != "/app/board.db" {
		t.Fatalf("sqlite row wrong: %+v", sq)
	}
}

func TestDatabaseCatalogOnOldAgentsIsJustTheContainers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/databases" {
			w.Write([]byte(`{"databases":[{"id":"c1","name":"shop","type":"postgres"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	res, err := c.GetDatabaseCatalog()
	if err != nil || len(res.Databases) != 1 {
		t.Fatalf("got %+v %v", res, err)
	}
}

func TestScanDatabasesReportsWhatItFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/databases/scan" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"added":["board"],"existing":2}`))
	}))
	defer srv.Close()
	c := NewClient()
	c.SetBaseURL(srv.URL)
	added, existing, err := c.ScanDatabases()
	if err != nil || len(added) != 1 || existing != 2 {
		t.Fatalf("%v %d %v", added, existing, err)
	}
}
