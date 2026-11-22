package handlers

import (
	"context"
	"html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"docklite-agent/internal/demo"
)

// The page visitors see when a site's container is stopped. nginx answers a closed port with a plain "502 Bad Gateway"; every
// site's nginx config includes a snippet that swaps that for this page (sent as a 503 "back soon"). One shared file
// serves every site, so a contact line set once in Settings shows on all of them.
//
// Visitors see it, so like the new-site page it names no server paths and loads nothing from elsewhere.

const (
	downPagePath    = "/var/lib/docklite/down.html"
	downSnippetPath = "/etc/nginx/snippets/docklite-down.conf"
	// A wildcard include matches nothing (instead of failing nginx's config test) while the snippet file is missing,
	// so a site config is safe whether or not the snippet exists yet.
	downIncludeLine = "include /etc/nginx/snippets/docklite-down*.conf;"

	downContactSetting = "down_page_contact"
	maxDownContactLen  = 200
)

const downPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<meta http-equiv="refresh" content="60">
<title>This site isn't running</title>
<style>
  :root {
    --pink: #FF6AD5; --coral: #FF9A8B; --gold: #FFD166; --mint: #B8F2A2; --sky: #9AD0FF; --lilac: #C7A4FF;
    --rainbow: linear-gradient(110deg, var(--pink), var(--coral), var(--gold), var(--mint), var(--sky), var(--lilac));
  }
  * { container-sizing: border-container; }
  html, body { height: 100%; margin: 0; }
  body {
    display: flex; align-items: center; justify-content: center; padding: 24px;
    font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, "Liberation Mono", monospace;
    color: #e9e4f5;
    background:
      radial-gradient(60rem 40rem at 15% 10%, rgba(255,106,213,.18), transparent 60%),
      radial-gradient(50rem 40rem at 90% 90%, rgba(154,208,255,.16), transparent 60%),
      linear-gradient(160deg, #0b0b16 0%, #140b24 55%, #0b1426 100%);
  }
  .card {
    width: 100%; max-width: 640px; padding: 40px 36px; border-radius: 22px; position: relative;
    background: rgba(22, 14, 34, .72); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px);
    container-shadow: 0 20px 60px rgba(0,0,0,.55), 0 0 40px rgba(199,164,255,.18);
  }
  .card::before {
    content: ""; position: absolute; inset: 0; border-radius: inherit; padding: 1.5px; background: var(--rainbow);
    -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); -webkit-mask-composite: xor;
    mask: linear-gradient(#000 0 0) content-box exclude, linear-gradient(#000 0 0); opacity: .7; pointer-events: none;
  }
  .state { display: inline-flex; align-items: center; gap: 8px; font-size: 12px; letter-spacing: .18em; text-transform: uppercase; color: var(--gold); }
  .state i { width: 9px; height: 9px; border-radius: 50%; background: var(--gold); container-shadow: 0 0 10px var(--gold); animation: pulse 2.4s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: .35; } }
  h1 {
    margin: 18px 0 14px; font-size: clamp(24px, 5.5vw, 38px); line-height: 1.2;
    background: var(--rainbow); -webkit-background-clip: text; background-clip: text; color: transparent;
    filter: drop-shadow(0 0 14px rgba(255,106,213,.35));
  }
  p { margin: 0 0 18px; color: #c9c1de; font-size: 15px; line-height: 1.6; }
  .contact { padding: 14px 16px; border-radius: 12px; background: rgba(199,164,255,.1); color: #e9e4f5; font-size: 14px; }
  .contact a { color: var(--sky); }
  footer { margin-top: 26px; padding-top: 18px; border-top: 1px solid rgba(199,164,255,.2); font-size: 12px; color: #9c93b4; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
  footer a { color: var(--lilac); text-decoration: none; }
  footer a:hover { text-decoration: underline; }
  @media (prefers-reduced-motion: reduce) { .state i { animation: none; } }
</style>
</head>
<body>
<main class="card">
  <span class="state"><i></i> Not running</span>
  <h1>This website's container isn't running right now</h1>
  <p>This site is hosted with DockLite, but the container it runs in is stopped. Nothing is wrong with your connection, and this page checks again every minute.</p>
  <p class="contact">{{CONTACT}}</p>
  <footer><span>Error 503</span><span>Hosted with <a href="https://docklite.net" rel="noopener">DockLite</a></span></footer>
</main>
</body>
</html>
`

var simpleEmail = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// cleanDownContact trims the contact line and keeps it to one short line of plain text.
func cleanDownContact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxDownContactLen {
		s = strings.TrimSpace(s[:maxDownContactLen])
	}
	return s
}

// downPageHTML makes the page. contact is what an admin typed (a name, an email, a link or a sentence); it is escaped,
// and a bare email address or web link becomes a link.
func downPageHTML(contact string) string {
	contact = cleanDownContact(contact)
	var line string
	switch {
	case contact == "":
		line = "If this is your site, start its container from your DockLite dashboard. Otherwise, please contact whoever runs this server."
	case simpleEmail.MatchString(contact):
		e := html.EscapeString(contact)
		line = `If this is your site, start its container from your DockLite dashboard. Otherwise, contact the administrator: <a href="mailto:` + e + `">` + e + `</a>`
	case strings.HasPrefix(contact, "https://") || strings.HasPrefix(contact, "http://"):
		e := html.EscapeString(contact)
		line = `If this is your site, start its container from your DockLite dashboard. Otherwise, contact the administrator: <a href="` + e + `" rel="noopener">` + e + `</a>`
	default:
		line = "If this is your site, start its container from your DockLite dashboard. Otherwise, contact the administrator: " + html.EscapeString(contact)
	}
	return strings.Replace(downPageTemplate, "{{CONTACT}}", line, 1)
}

var downPageMu sync.Mutex

// EnsureDownPage writes the page (and the nginx snippet that serves it) when what is on disk differs from what it
// should be, or when force is set. Safe to call often: a matching page and snippet cost two file reads.
func (h *Handlers) EnsureDownPage(force bool) error {
	if demo.On {
		return nil
	}
	downPageMu.Lock()
	defer downPageMu.Unlock()
	contact, err := h.store.GetSetting(downContactSetting)
	if err != nil {
		return err
	}
	want := downPageHTML(contact)
	if !force {
		have, _ := os.ReadFile(downPagePath)
		if string(have) == want {
			if _, err := os.Stat(downSnippetPath); err == nil {
				return nil
			}
		}
	}
	out, err := runRootHelper(strings.NewReader(want), "nginx-down-write")
	if err != nil {
		return &downPageError{strings.TrimSpace(string(out))}
	}
	return nil
}

type downPageError struct{ msg string }

func (e *downPageError) Error() string {
	if e.msg == "" {
		return "could not write the stopped-container page"
	}
	return e.msg
}

// SyncDownPage makes sure the page exists and that every site's nginx server block includes the snippet. It only
// ever adds one include line, in server blocks that name a site DockLite knows and proxy to a local upstream; the
// helper backs the file up, tests nginx's config and puts the file back if nginx rejects it.
func (h *Handlers) SyncDownPage(ctx context.Context) {
	if demo.On {
		return
	}
	if err := h.EnsureDownPage(false); err != nil {
		log.Printf("stopped-container page: %v", err)
		return
	}
	sites, err := h.store.ListSites()
	if err != nil || len(sites) == 0 {
		return
	}
	domains := map[string]bool{}
	for _, s := range sites {
		domains[strings.ToLower(s.Domain)] = true
	}
	listing, err := runRootHelper(nil, "nginx-upstreams")
	if err != nil {
		return
	}
	for _, p := range planDownPatches(parseUpstreamBlocks(string(listing)), domains) {
		msg, perr := runRootHelper(nil, "nginx-down-patch", p.File, p.Domain)
		detail := map[string]any{"file": p.File, "ok": perr == nil}
		if perr != nil {
			detail["error"] = strings.TrimSpace(string(msg))
			log.Printf("stopped-container page: could not add it for %s in %s: %s", p.Domain, p.File, strings.TrimSpace(string(msg)))
		} else {
			log.Printf("stopped-container page: added for %s", p.Domain)
		}
		auditSystem("nginx.down-page", p.Domain, detail)
	}
}

type downPatch struct{ File, Domain string }

// planDownPatches picks the server blocks to add the include to: ones that name a known site, proxy to a local
// upstream and don't have it (or their own 502 page) yet. A listing from an older helper has no such column; then
// nothing is planned, because we can't tell which blocks already have it.
func planDownPatches(blocks []upstreamBlock, sites map[string]bool) []downPatch {
	var out []downPatch
	done := map[string]bool{}
	for _, b := range blocks {
		if !b.DownKnown || b.HasDown || len(b.Ports) == 0 {
			continue
		}
		for _, name := range b.Names {
			domain := strings.TrimPrefix(name, "www.")
			if !sites[domain] {
				continue
			}
			if key := b.File + "|" + domain; !done[key] {
				done[key] = true
				out = append(out, downPatch{File: b.File, Domain: domain})
			}
			break
		}
	}
	return out
}

// DownPageSettings is /api/settings/down-page. GET returns the contact line and the page as it will look; PUT (admin)
// changes the contact line and rewrites the page.
func (h *Handlers) DownPageSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		contact, err := h.store.GetSetting(downContactSetting)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contact": contact, "html": downPageHTML(contact), "max_length": maxDownContactLen})
	case http.MethodPut:
		if !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		var body struct {
			Contact string `json:"contact"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		contact := cleanDownContact(body.Contact)
		if err := h.store.SetSetting(downContactSetting, contact); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.EnsureDownPage(true); err != nil {
			writeError(w, http.StatusInternalServerError, "saved, but the page could not be written: "+err.Error())
			return
		}
		h.audit(r, "settings.down-page", "contact", nil)
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "contact": contact, "html": downPageHTML(contact)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
