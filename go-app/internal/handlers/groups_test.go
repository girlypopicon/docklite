package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func groupFixture(t *testing.T) (*Handlers, *store.SQLiteStore, *store.UserRecord, *store.UserRecord) {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	testhelpers.AssertNoError(t, s.EnsureGroupSchema())
	testhelpers.AssertNoError(t, s.EnsureSettingsSchema())
	alice, err := s.CreateUser("alice", "alicepassword1", "user", nil)
	testhelpers.AssertNoError(t, err)
	bob, err := s.CreateUser("bob", "bobpassword12", "user", nil)
	testhelpers.AssertNoError(t, err)
	return &Handlers{store: s}, s, alice, bob
}

func asRole(r *http.Request, id int64, role string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserIDKey, id)
	ctx = context.WithValue(ctx, ctxUserRoleKey, role)
	return r.WithContext(ctx)
}

func TestGroupStore(t *testing.T) {
	_, s, alice, bob := groupFixture(t)

	g, err := s.CreateGroup(alice.ID, "  Blog  ")
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, g.Name, "Blog")
	if g.Linked {
		t.Fatal("a new group isn't linked")
	}
	if _, err := s.CreateGroup(alice.ID, "blog"); err != store.ErrGroupNameTaken {
		t.Fatalf("same name, different case, same owner should clash: %v", err)
	}
	if _, err := s.CreateGroup(bob.ID, "blog"); err != nil {
		t.Fatalf("another owner may reuse the name: %v", err)
	}

	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "docklite-site-blog-com"))
	testhelpers.AssertNoError(t, s.AddToGroup(g.ID, "docklite-db-blog"))
	g2, _ := s.CreateGroup(alice.ID, "Other")
	// a box is in one group: moving it takes it out of the first
	testhelpers.AssertNoError(t, s.AddToGroup(g2.ID, "docklite-db-blog"))
	got, _ := s.GetGroup(g.ID)
	testhelpers.AssertEqual(t, strings.Join(got.Members, ","), "docklite-site-blog-com")
	got2, _ := s.GetGroup(g2.ID)
	testhelpers.AssertEqual(t, strings.Join(got2.Members, ","), "docklite-db-blog")

	linked := true
	testhelpers.AssertNoError(t, s.UpdateGroup(g.ID, nil, &linked, nil, nil))
	got, _ = s.GetGroup(g.ID)
	testhelpers.AssertEqual(t, got.Linked, true)
	newName := "Other"
	if err := s.UpdateGroup(g.ID, &newName, nil, nil, nil); err != store.ErrGroupNameTaken {
		t.Fatalf("renaming onto an existing name should clash: %v", err)
	}

	mine, _ := s.ListGroups(alice.ID)
	testhelpers.AssertEqual(t, len(mine), 2)
	all, _ := s.ListGroups(0)
	testhelpers.AssertEqual(t, len(all), 3)

	// deleting a group frees its boxes (they aren't deleted)
	testhelpers.AssertNoError(t, s.DeleteGroup(g2.ID))
	in, _ := s.GroupOfBox("docklite-db-blog")
	if in != nil {
		t.Fatal("box should be back on the desktop")
	}
}

func TestGroupAPIPermissionsAndValidation(t *testing.T) {
	h, s, alice, bob := groupFixture(t)
	ag, _ := s.CreateGroup(alice.ID, "Alice's")

	call := func(r *http.Request) (int, map[string]any) {
		rr := httptest.NewRecorder()
		h.GroupRoutes(rr, r)
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr.Code, out
	}
	mk := func(method, path, body string, uid int64, role string) *http.Request {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return asRole(r, uid, role)
	}

	// each person lists only their own groups; an admin sees all
	code, out := call(mk(http.MethodGet, "/api/groups", "", bob.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusOK)
	testhelpers.AssertEqual(t, len(out["groups"].([]any)), 0)
	_, out = call(mk(http.MethodGet, "/api/groups", "", alice.ID, "user"))
	testhelpers.AssertEqual(t, len(out["groups"].([]any)), 1)
	_, out = call(mk(http.MethodGet, "/api/groups", "", bob.ID, "admin"))
	testhelpers.AssertEqual(t, len(out["groups"].([]any)), 1)

	path := "/api/groups/" + itoa(ag.ID)
	// someone else's group can't be changed or deleted
	code, _ = call(mk(http.MethodPut, path, `{"linked":true}`, bob.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusForbidden)
	code, _ = call(mk(http.MethodDelete, path, "", bob.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusForbidden)

	// the owner can; values are checked
	code, out = call(mk(http.MethodPut, path, `{"linked":true,"color":"mint","icon":"heart","name":"Shop"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusOK)
	g := out["group"].(map[string]any)
	testhelpers.AssertEqual(t, g["linked"], true)
	testhelpers.AssertEqual(t, g["name"], "Shop")
	code, _ = call(mk(http.MethodPut, path, `{"color":"chartreuse"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusBadRequest)
	code, _ = call(mk(http.MethodPut, path, `{"icon":"<script>"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusBadRequest)
	code, _ = call(mk(http.MethodPut, path, `{"name":"a/b"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusBadRequest)

	// creating needs a sensible name, and a clash is a 409
	code, _ = call(mk(http.MethodPost, "/api/groups", `{"name":""}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusBadRequest)
	code, _ = call(mk(http.MethodPost, "/api/groups", `{"name":"shop"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusConflict)
	code, _ = call(mk(http.MethodPost, "/api/groups", `{"name":"`+strings.Repeat("x", 51)+`"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusBadRequest)
	code, _ = call(mk(http.MethodPost, "/api/groups", `{"name":"Café ☕ & friends"}`, alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusOK)

	code, _ = call(mk(http.MethodDelete, path, "", alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusOK)
	code, _ = call(mk(http.MethodGet, "/api/groups/99999", "", alice.ID, "user"))
	testhelpers.AssertEqual(t, code, http.StatusNotFound)
}

func TestMigrateFoldersToGroups(t *testing.T) {
	_, s, alice, bob := groupFixture(t)
	db := s.DB

	exec := func(q string, args ...any) int64 {
		res, err := db.Exec(q, args...)
		testhelpers.AssertNoError(t, err)
		id, _ := res.LastInsertId()
		return id
	}
	// alice: Default (dropped), "Clients" with two boxes (one gone), a nested "Clients/Acme"; bob: "Clients" too (name clash is per owner)
	def := exec(`INSERT INTO folders (user_id, name, depth, position) VALUES (?, 'Default', 0, 0)`, alice.ID)
	clients := exec(`INSERT INTO folders (user_id, name, depth, position) VALUES (?, 'Clients', 0, 1)`, alice.ID)
	acme := exec(`INSERT INTO folders (user_id, name, parent_folder_id, depth, position) VALUES (?, 'Acme', ?, 1, 0)`, alice.ID, clients)
	bobs := exec(`INSERT INTO folders (user_id, name, depth, position) VALUES (?, 'Clients', 0, 0)`, bob.ID)
	for i, c := range []struct {
		f  int64
		id string
	}{{def, "id-default"}, {clients, "id-a"}, {clients, "id-gone"}, {acme, "id-b"}, {bobs, "id-c"}} {
		exec(`INSERT INTO folder_containers (folder_id, container_id, position) VALUES (?, ?, ?)`, c.f, c.id, i)
	}

	n, err := migrateFoldersToGroups(s, map[string]string{"id-default": "box-default", "id-a": "box-a", "id-b": "box-b", "id-c": "box-c"})
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, n, 3) // Clients, Acme, and bob's Clients: no Default

	mine, _ := s.ListGroups(alice.ID)
	names := map[string]string{}
	for _, g := range mine {
		names[g.Name] = strings.Join(g.Members, ",")
	}
	testhelpers.AssertEqual(t, len(mine), 2)
	testhelpers.AssertEqual(t, names["Clients"], "box-a") // the box that no longer exists is skipped
	testhelpers.AssertEqual(t, names["Acme"], "box-b")    // nested became top level
	if g, _ := s.GroupOfBox("box-default"); g != nil {
		t.Fatal("what was in Default is just on the desktop now")
	}
	theirs, _ := s.ListGroups(bob.ID)
	testhelpers.AssertEqual(t, len(theirs), 1)
	testhelpers.AssertEqual(t, theirs[0].Name, "Clients")
}
