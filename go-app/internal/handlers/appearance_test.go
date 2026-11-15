package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestAuthAppearanceSavesPerUser(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	testhelpers.AssertNoError(t, s.InitializeAgentTables())
	h := &Handlers{store: s}

	call := func(userID int64, method, body string) (int, string) {
		req := httptest.NewRequest(method, "/api/auth/appearance", strings.NewReader(body))
		if userID > 0 {
			req = req.WithContext(context.WithValue(req.Context(), ctxUserIDKey, userID))
		}
		rec := httptest.NewRecorder()
		h.AuthAppearance(rec, req)
		return rec.Code, strings.TrimSpace(rec.Body.String())
	}

	code, body := call(1, http.MethodGet, "")
	testhelpers.AssertEqual(t, http.StatusOK, code)
	testhelpers.AssertEqual(t, `{"appearance":null}`, body)

	code, _ = call(1, http.MethodPut, `{"theme":"unicorn","fontSize":"large","animations":false,"glow":120}`)
	testhelpers.AssertEqual(t, http.StatusOK, code)
	code, body = call(1, http.MethodGet, "")
	testhelpers.AssertEqual(t, http.StatusOK, code)
	testhelpers.AssertEqual(t, `{"appearance":{"theme":"unicorn","fontSize":"large","animations":false,"glow":120}}`, body)

	// another user has their own
	_, body = call(2, http.MethodGet, "")
	testhelpers.AssertEqual(t, `{"appearance":null}`, body)

	code, _ = call(1, http.MethodPut, `{"theme":"<script>","fontSize":"large","glow":0}`)
	testhelpers.AssertEqual(t, http.StatusBadRequest, code)
	code, _ = call(1, http.MethodPut, `{"theme":"corpo","fontSize":"medium","glow":999}`)
	testhelpers.AssertEqual(t, http.StatusBadRequest, code)
	code, _ = call(0, http.MethodGet, "")
	testhelpers.AssertEqual(t, http.StatusUnauthorized, code)
}
