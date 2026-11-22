package handlers

import (
	"strings"
	"testing"
)

func TestMaintenancePageHTML(t *testing.T) {
	page := maintenancePageHTML(`We're moving <b>servers</b>`, "help@example.com")
	for _, want := range []string{"Back soon", "We&#39;re moving &lt;b&gt;servers&lt;/b&gt;", "help@example.com", "Error 503"} {
		if !strings.Contains(page, want) {
			t.Errorf("page should contain %q", want)
		}
	}
	if strings.Contains(page, "<b>servers</b>") {
		t.Fatal("the message must be escaped")
	}
	if strings.Contains(page, "isn't running") || strings.Contains(page, "start its container") {
		t.Fatal("must not carry the stopped-container wording")
	}
	if !strings.Contains(maintenancePageHTML("", ""), "being updated") {
		t.Fatal("empty message falls back to the default")
	}
	if len(cleanMaintenanceMessage(strings.Repeat("x ", 400))) > maxMaintenanceMessageLen {
		t.Fatal("message must be capped")
	}
}
