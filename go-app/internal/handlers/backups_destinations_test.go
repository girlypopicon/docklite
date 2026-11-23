package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func destHandlers(t *testing.T) *Handlers {
	t.Helper()
	h := testHandlers(t)
	testhelpers.AssertNoError(t, h.store.EnsureBackupPackageSchema())
	testhelpers.AssertNoError(t, h.store.EnsureRemoteServerSchema())
	h.backupBaseDir = t.TempDir()
	return h
}

func TestDestinationSecretsNeverLeaveTheServer(t *testing.T) {
	h := destHandlers(t)
	body := `{"name":"R2","type":"s3","config":{"endpoint":"https://acct.r2.cloudflarestorage.com","region":"auto","bucket":"bk","access_key":"AK","secret_key":"TOPSECRET"}}`
	code, out := adminJSON(t, h.BackupDestinations, "POST", "/api/backups/destinations", body)
	testhelpers.AssertEqual(t, 200, code)
	id := int64(out["id"].(float64))

	req := httptest.NewRequest("GET", "/api/backups/destinations", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxUserRoleKey, "admin"))
	rec := httptest.NewRecorder()
	h.BackupDestinations(rec, req)
	testhelpers.AssertTrue(t, !strings.Contains(rec.Body.String(), "TOPSECRET"), "the secret key was sent to the browser: "+rec.Body.String())
	testhelpers.AssertTrue(t, strings.Contains(rec.Body.String(), "secret_key_set"), "the list should say a secret is set")

	// editing without retyping the secret keeps it
	code, _ = adminJSON(t, h.BackupDestinations, "PUT", "/api/backups/destinations", `{"id":`+itoa(id)+`,"config":{"endpoint":"https://acct.r2.cloudflarestorage.com","region":"auto","bucket":"bk2","access_key":"AK","secret_key":""}}`)
	testhelpers.AssertEqual(t, 200, code)
	d, _ := h.store.GetBackupDestinationByID(id)
	cfg := backup.ParseDestConfig(d.Config)
	testhelpers.AssertEqual(t, "TOPSECRET", cfg.SecretKey)
	testhelpers.AssertEqual(t, "bk2", cfg.Bucket)

	// the type can't change under saved backups
	code, _ = adminJSON(t, h.BackupDestinations, "PUT", "/api/backups/destinations", `{"id":`+itoa(id)+`,"type":"local"}`)
	testhelpers.AssertEqual(t, 400, code)
}

func TestDestinationValidationAndEncryptionNeedsAKey(t *testing.T) {
	h := destHandlers(t)
	for _, body := range []string{
		`{"name":"x","type":"s3","config":{"bucket":"bk"}}`,
		`{"name":"x","type":"ftp","config":{}}`,
		`{"name":"x","type":"local","config":{"path":"/tmp/x","encrypt":true}}`,
		`{"name":"x","type":"server","config":{"server_id":99}}`,
	} {
		code, out := adminJSON(t, h.BackupDestinations, "POST", "/api/backups/destinations", body)
		testhelpers.AssertEqual(t, 400, code)
		testhelpers.AssertTrue(t, out["error"] != nil, "should explain: "+body)
	}
	code, key := adminJSON(t, h.BackupKeys, "POST", "/api/backups/keys", "")
	testhelpers.AssertEqual(t, 200, code)
	testhelpers.AssertTrue(t, strings.HasPrefix(key["recipient"].(string), "age1"), "recipient")
	testhelpers.AssertTrue(t, strings.HasPrefix(key["identity"].(string), "AGE-SECRET-KEY-"), "identity")
	code, _ = adminJSON(t, h.BackupDestinations, "POST", "/api/backups/destinations",
		`{"name":"x","type":"local","config":{"path":"`+t.TempDir()+`","encrypt":true,"recipient":"`+key["recipient"].(string)+`"}}`)
	testhelpers.AssertEqual(t, 200, code)
	// the private half is never stored
	dests, _ := h.store.GetBackupDestinations()
	for _, d := range dests {
		testhelpers.AssertTrue(t, !strings.Contains(d.Config, "AGE-SECRET-KEY"), "private key stored: "+d.Config)
	}
}

func TestOnlyAdminsCanUseBackupDestinations(t *testing.T) {
	h := destHandlers(t)
	for name, fn := range map[string]http.HandlerFunc{"keys": h.BackupKeys, "test": h.BackupDestinationTest, "fetch": h.BackupFetch, "incoming": h.BackupIncoming} {
		req := httptest.NewRequest("POST", "/x", strings.NewReader("{}"))
		req = req.WithContext(context.WithValue(req.Context(), ctxUserRoleKey, "user"))
		rec := httptest.NewRecorder()
		fn(rec, req)
		testhelpers.AssertEqual(t, 403, rec.Code)
		_ = name
	}
}

func TestDestinationTestReportsProblemsInPlainWords(t *testing.T) {
	h := destHandlers(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`<Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
	}))
	defer srv.Close()
	code, out := adminJSON(t, h.BackupDestinationTest, "POST", "/api/backups/destinations/test",
		`{"type":"s3","config":{"endpoint":"`+srv.URL+`","region":"auto","bucket":"bk","access_key":"AK","secret_key":"SK"}}`)
	testhelpers.AssertEqual(t, 200, code)
	testhelpers.AssertEqual(t, false, out["ok"])
	testhelpers.AssertTrue(t, strings.Contains(out["error"].(string), "AccessDenied"), out["error"].(string))
}

func TestIncomingBackupsBetweenServers(t *testing.T) {
	// the second server: its agent stores what arrives
	second := destHandlers(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer hubtoken" {
			w.WriteHeader(401)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxUserRoleKey, "admin"))
		second.BackupIncoming(w, r)
	}))
	defer srv.Close()

	// the first server knows the second and sends to it, locked
	first := destHandlers(t)
	rs, err := first.store.AddRemoteServer("second", srv.URL, "hubtoken")
	testhelpers.AssertNoError(t, err)
	rcpt, ident, _ := backup.GenerateKey()
	_ = ident
	destID, err := first.store.CreateBackupDestination("Second server", "server", `{"server_id":`+itoa(rs.ID)+`,"encrypt":true,"recipient":"`+rcpt+`"}`, 1)
	testhelpers.AssertNoError(t, err)
	dest, _ := first.store.GetBackupDestinationByID(destID)
	remote, err := backup.OpenRemote(first.store, *dest)
	testhelpers.AssertNoError(t, err)
	msg, err := remote.Test(context.Background())
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertTrue(t, strings.Contains(msg, "second"), msg)

	src := filepath.Join(t.TempDir(), "b.litpkg.age")
	testhelpers.AssertNoError(t, os.WriteFile(src, []byte("locked bytes"), 0o600))
	ref, err := remote.Put(context.Background(), src, "packages/b-123.litpkg.age")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, "server://"+itoa(rs.ID)+"/b-123.litpkg.age", ref)
	stored, _ := os.ReadFile(filepath.Join(second.backupBaseDir, incomingDir, "b-123.litpkg.age"))
	testhelpers.AssertEqual(t, "locked bytes", string(stored))

	back := filepath.Join(t.TempDir(), "back")
	testhelpers.AssertNoError(t, remote.Get(context.Background(), ref, back))
	got, _ := os.ReadFile(back)
	testhelpers.AssertEqual(t, "locked bytes", string(got))

	testhelpers.AssertNoError(t, remote.Delete(context.Background(), ref))
	_, err = os.Stat(filepath.Join(second.backupBaseDir, incomingDir, "b-123.litpkg.age"))
	testhelpers.AssertTrue(t, os.IsNotExist(err), "the file should be deleted on the second server")

	// a wrong token is refused, and only backup file names are accepted
	bad, _ := first.store.AddRemoteServer("wrong", srv.URL, "nope")
	_, err = backup.OpenRemote(first.store, store.BackupDestination{Type: "server", Config: `{"server_id":` + itoa(bad.ID) + `}`})
	testhelpers.AssertNoError(t, err)
	hr, _ := backup.OpenRemote(first.store, store.BackupDestination{Type: "server", Config: `{"server_id":` + itoa(bad.ID) + `}`})
	_, err = hr.Test(context.Background())
	testhelpers.AssertTrue(t, err != nil, "a wrong token must be refused")
	for _, n := range []string{"../../etc/passwd", "notes.txt", "x.sh"} {
		if name, ok := incomingName(n); ok {
			t.Errorf("accepted %q as %q", n, name)
		}
	}
	if name, ok := incomingName("../../a/b.litpkg"); !ok || strings.Contains(name, "/") {
		t.Errorf("a path should reduce to a plain name, got %q %v", name, ok)
	}
}
