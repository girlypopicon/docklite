package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func TestBoxStyleRejectsUnknownValues(t *testing.T) {
	h := &Handlers{}
	for _, tc := range []struct {
		method, body string
		want         int
	}{
		{http.MethodGet, `{}`, http.StatusMethodNotAllowed},
		{http.MethodPut, `{"color":"chartreuse"}`, http.StatusBadRequest},
		{http.MethodPut, `{"icon":"<script>"}`, http.StatusBadRequest},
		{http.MethodPut, `{"color":"pink","icon":"../../etc"}`, http.StatusBadRequest},
		{http.MethodPut, `not json`, http.StatusBadRequest},
	} {
		rr := httptest.NewRecorder()
		h.SetContainerStyle(rr, httptest.NewRequest(tc.method, "/x", strings.NewReader(tc.body)), "abc123")
		testhelpers.AssertEqual(t, rr.Code, tc.want)
	}
}

func TestBoxStyleStore(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	defer s.Close()
	testhelpers.AssertNoError(t, s.EnsureBoxStyleSchema())

	testhelpers.AssertNoError(t, s.SetBoxStyle("docklite-site-a-com", store.BoxStyle{Color: "pink", Icon: "heart"}))
	testhelpers.AssertNoError(t, s.SetBoxStyle("docklite-site-b-com", store.BoxStyle{Icon: "cat"}))
	got, err := s.ListBoxStyles()
	testhelpers.AssertNoError(t, err)
	testhelpers.AssertEqual(t, got["docklite-site-a-com"], store.BoxStyle{Color: "pink", Icon: "heart"})
	testhelpers.AssertEqual(t, got["docklite-site-b-com"], store.BoxStyle{Icon: "cat"})

	// changing it replaces it; the defaults (both empty) remove the row
	testhelpers.AssertNoError(t, s.SetBoxStyle("docklite-site-a-com", store.BoxStyle{Color: "mint"}))
	testhelpers.AssertNoError(t, s.SetBoxStyle("docklite-site-b-com", store.BoxStyle{}))
	got, _ = s.ListBoxStyles()
	testhelpers.AssertEqual(t, len(got), 1)
	testhelpers.AssertEqual(t, got["docklite-site-a-com"], store.BoxStyle{Color: "mint"})
}
