package handlers

import (
	"testing"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func testHandlers(t *testing.T) *Handlers {
	t.Helper()
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	t.Cleanup(func() { s.Close() })
	testhelpers.AssertNoError(t, s.EnsureSettingsSchema())
	return &Handlers{store: s}
}

func TestAutoRenewMasterSwitchAndPerDomainOverrides(t *testing.T) {
	h := testHandlers(t)
	if !h.autoRenewFor("a.com") || !h.autoRenewFor("b.com") {
		t.Fatal("certificates renew by default")
	}
	h.store.SetSetting(autoRenewDomainPrefix+"a.com", "off")
	if h.autoRenewFor("a.com", "www.a.com") {
		t.Fatal("a domain switched off must not renew")
	}
	if !h.autoRenewFor("b.com") {
		t.Fatal("switching one domain off must not affect another")
	}
	h.store.SetSetting(settingSSLAutoRenewDefault, "off")
	if h.autoRenewFor("b.com") {
		t.Fatal("none in Settings turns renewal off everywhere")
	}
	h.store.SetSetting(settingSSLAutoRenewDefault, "on")
	if h.autoRenewFor("a.com") || !h.autoRenewFor("b.com") {
		t.Fatal("going back to all keeps each domain's own switch")
	}
}

func TestDueForRenewalNeedsUnder30DaysAndSwitchOn(t *testing.T) {
	h := testHandlers(t)
	days := func(n int) *int { return &n }
	if h.dueForRenewal(sslCertInfo{Domain: "a.com", DaysUntilExpiry: days(45)}) {
		t.Fatal("45 days left is not due")
	}
	if !h.dueForRenewal(sslCertInfo{Domain: "a.com", DaysUntilExpiry: days(9)}) {
		t.Fatal("9 days left is due")
	}
	if !h.dueForRenewal(sslCertInfo{Domain: "a.com", DaysUntilExpiry: days(-2)}) {
		t.Fatal("an expired certificate is due")
	}
	h.store.SetSetting(autoRenewDomainPrefix+"a.com", "off")
	if h.dueForRenewal(sslCertInfo{Domain: "a.com", DaysUntilExpiry: days(9)}) {
		t.Fatal("switched off is never due")
	}
}

func TestRenewalFailureIsRemembered(t *testing.T) {
	h := testHandlers(t)
	h.saveRenewal("a.com", renewalRecord{At: time.Now().UTC().Format(time.RFC3339), OK: false, Error: "dns problem"})
	rec := h.lastRenewal("a.com", "www.a.com")
	if rec == nil || rec.OK || rec.Error != "dns problem" {
		t.Fatalf("got %+v", rec)
	}
}
