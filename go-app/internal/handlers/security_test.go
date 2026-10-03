package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestIsValidDomainRejectsNginxInjection(t *testing.T) {
	valid := []string{"example.com", "www.example.com", "a-b.example.co.uk", "x1.io"}
	for _, d := range valid {
		testhelpers.AssertTrue(t, isValidDomain(d), "expected valid: "+d)
	}

	invalid := []string{
		"", "..", ".", "-example.com", "example-.com", "exa mple.com",
		"example.com;", "evil.com; location / { return 200; }", "a.com\nserver {",
		"a/b.com", `a\b.com`, "a..com", strings.Repeat("a", 64) + ".com",
	}
	for _, d := range invalid {
		testhelpers.AssertFalse(t, isValidDomain(d), "expected invalid: "+d)
	}
}

func TestSetupNginxForDomainRejectsInvalidDomain(t *testing.T) {
	err := setupNginxForDomain("evil.com; include x", false, 8080)
	testhelpers.AssertErrorContains(t, err, "invalid domain")
}

func TestUserPasswordAdminCannotResetSuperAdmin(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	h := &Handlers{store: s}

	superAdmin, err := s.CreateUser("root", "superpassword1", "super_admin", nil)
	testhelpers.AssertNoError(t, err)
	admin, err := s.CreateUser("ops", "adminpassword1", "admin", &superAdmin.ID)
	testhelpers.AssertNoError(t, err)
	plain, err := s.CreateUser("bob", "userpassword1", "user", &admin.ID)
	testhelpers.AssertNoError(t, err)

	reset := func(actor *store.UserRecord, role string, targetID int64) int {
		body := `{"userId":` + strconv.FormatInt(targetID, 10) + `,"newPassword":"brandnewpass1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/users/password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), ctxUserIDKey, actor.ID)
		ctx = context.WithValue(ctx, ctxUserRoleKey, role)
		rec := httptest.NewRecorder()
		h.UserPassword(rec, req.WithContext(ctx))
		return rec.Code
	}

	testhelpers.AssertEqual(t, http.StatusForbidden, reset(admin, "admin", superAdmin.ID))
	testhelpers.AssertEqual(t, http.StatusOK, reset(admin, "admin", plain.ID))
	testhelpers.AssertEqual(t, http.StatusOK, reset(superAdmin, "super_admin", admin.ID))
}
