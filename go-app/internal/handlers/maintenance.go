package handlers

import (
	"html"
	"net/http"
	"strings"

	"docklite-agent/internal/demo"
)

// "Back soon" mode: while it is on for a site (or for every site), visitors get a branded page with a 503 and a
// Retry-After header instead of the site. It reuses the stopped-container page's look. The nginx side is one snippet
// (see maintenance-write in the root helper) plus one flag file per site, so switching it on or off reloads nothing.

const (
	maintenanceMessageSetting = "maintenance_message"
	maxMaintenanceMessageLen  = 300
	defaultMaintenanceMessage = "This website is being updated and will be back shortly. Nothing is wrong with your connection; please try again in a little while."
)

func cleanMaintenanceMessage(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxMaintenanceMessageLen {
		s = strings.TrimSpace(s[:maxMaintenanceMessageLen])
	}
	return s
}

// maintenancePageHTML builds the page from the stopped-container page, so both look alike.
func maintenancePageHTML(message, contact string) string {
	message = cleanMaintenanceMessage(message)
	if message == "" {
		message = defaultMaintenanceMessage
	}
	page := downPageHTML(contact)
	// downPageHTML's own contact line talks about starting a container; keep only the administrator's contact.
	contactLine := ""
	if c := cleanDownContact(contact); c != "" {
		contactLine = `<p class="contact">Questions? Contact the administrator: ` + html.EscapeString(c) + `</p>`
	}
	start := strings.Index(page, `<main class="card">`)
	end := strings.Index(page, `</main>`)
	if start < 0 || end < 0 {
		return page
	}
	body := `<main class="card">
  <span class="state"><i></i> Back soon</span>
  <h1>We'll be right back</h1>
  <p>` + html.EscapeString(message) + `</p>
  ` + contactLine + `
  <footer><span>Error 503</span><span>Hosted with <a href="https://docklite.net" rel="noopener">DockLite</a></span></footer>
`
	page = page[:start] + body + page[end:]
	page = strings.Replace(page, "<title>This site isn't running</title>", "<title>Back soon</title>", 1)
	return page
}

func (h *Handlers) maintenanceActive() []string {
	out, err := runRootHelper(nil, "maintenance-list")
	if err != nil {
		return []string{}
	}
	var names []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	if names == nil {
		names = []string{}
	}
	return names
}

// Maintenance: GET lists what is in back-soon mode; POST {domain, on, message} switches it (admin only). domain is a
// site's domain or "all".
func (h *Handlers) Maintenance(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		msg, _ := h.store.GetSetting(maintenanceMessageSetting)
		writeJSON(w, http.StatusOK, map[string]any{"active": h.maintenanceActive(), "message": msg, "defaultMessage": defaultMaintenanceMessage})
	case http.MethodPost:
		var body struct {
			Domain  string  `json:"domain"`
			On      bool    `json:"on"`
			Message *string `json:"message"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		name := strings.ToLower(strings.TrimSpace(body.Domain))
		if name == "all" {
			name = "ALL"
		} else if !isValidDomain(name) {
			writeError(w, http.StatusBadRequest, `give a domain, or "all"`)
			return
		}
		if demo.On {
			writeError(w, http.StatusForbidden, "not available in the demo")
			return
		}
		if body.Message != nil {
			if err := h.store.SetSetting(maintenanceMessageSetting, cleanMaintenanceMessage(*body.Message)); err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}
		if body.On {
			msg, _ := h.store.GetSetting(maintenanceMessageSetting)
			contact, _ := h.store.GetSetting(downContactSetting)
			if out, err := runRootHelper(strings.NewReader(maintenancePageHTML(msg, contact)), "maintenance-write"); err != nil {
				writeError(w, http.StatusInternalServerError, "could not install the back-soon page: "+strings.TrimSpace(string(out)))
				return
			}
		}
		mode := "off"
		if body.On {
			mode = "on"
		}
		if out, err := runRootHelper(nil, "maintenance-set", name, mode); err != nil {
			writeError(w, http.StatusInternalServerError, "could not change back-soon mode: "+strings.TrimSpace(string(out)))
			return
		}
		h.audit(r, "nginx.maintenance", name, map[string]any{"on": body.On})
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "active": h.maintenanceActive()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
