package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/demo"
)

// Automatic HTTPS: when a site is added, DockLite gets its Let's Encrypt certificate by itself.
//
// Let's Encrypt only issues a certificate once the domain's DNS points at this server, and a brand-new DNS record often
// hasn't spread yet. Asking too early fails, and repeated failures get the domain temporarily blocked. So each site gets
// a small background job: wait until DNS points here (or at Cloudflare, for a proxied domain), then ask once through the
// same root-helper route the manual button uses, and retry politely if that fails. Jobs are saved to disk and resume after
// a restart.

const (
	certWaiting = "waiting-dns"
	certIssuing = "issuing"
	certIssued  = "issued"
	certFailed  = "failed"

	autoCertFile     = "data/auto-certs.json"
	settingsFile     = "data/settings.json"
	dnsWaitLimit     = 48 * time.Hour
	certTickInterval = 15 * time.Second
)

type certJob struct {
	Domain         string    `json:"domain"`
	IncludeWww     bool      `json:"includeWww"`
	State          string    `json:"state"`
	Message        string    `json:"message"`
	FailedAttempts int       `json:"failedAttempts"`
	NextAttempt    time.Time `json:"nextAttempt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type certQueue struct {
	mu   sync.Mutex
	path string
	jobs map[string]*certJob
}

func newCertQueue(path string) *certQueue {
	q := &certQueue{path: path, jobs: map[string]*certJob{}}
	if data, err := os.ReadFile(path); err == nil {
		var list []*certJob
		if json.Unmarshal(data, &list) == nil {
			for _, j := range list {
				q.jobs[j.Domain] = j
			}
		}
	}
	return q
}

func (q *certQueue) saveLocked() {
	list := make([]*certJob, 0, len(q.jobs))
	for _, j := range q.jobs {
		list = append(list, j)
	}
	sort.Slice(list, func(i, k int) bool { return list[i].Domain < list[k].Domain })
	if data, err := json.MarshalIndent(list, "", " "); err == nil {
		_ = os.MkdirAll(filepath.Dir(q.path), 0o750)
		_ = os.WriteFile(q.path, data, 0o640)
	}
}

var autoCerts = newCertQueue(autoCertFile)

// enqueue adds (or restarts) a job. Returns the job.
func (q *certQueue) enqueue(domain string, includeWww bool, in time.Duration) certJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	j := q.jobs[domain]
	if j == nil {
		j = &certJob{Domain: domain, CreatedAt: now}
		q.jobs[domain] = j
	}
	j.IncludeWww, j.State, j.FailedAttempts = includeWww, certWaiting, 0
	j.Message = "Waiting for the domain's DNS to point at this server."
	j.NextAttempt, j.UpdatedAt = now.Add(in), now
	q.saveLocked()
	return *j
}

func (q *certQueue) snapshot() []certJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]certJob, 0, len(q.jobs))
	for _, j := range q.jobs {
		out = append(out, *j)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Domain < out[k].Domain })
	return out
}

// next returns a copy of the next job that is due, if any.
func (q *certQueue) next(now time.Time) (certJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var best *certJob
	for _, j := range q.jobs {
		if (j.State == certWaiting) && !j.NextAttempt.After(now) {
			if best == nil || j.NextAttempt.Before(best.NextAttempt) {
				best = j
			}
		}
	}
	if best == nil {
		return certJob{}, false
	}
	return *best, true
}

func (q *certQueue) update(domain string, fn func(j *certJob)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if j := q.jobs[domain]; j != nil {
		fn(j)
		j.UpdatedAt = time.Now()
		q.saveLocked()
	}
}

// ---- DNS ----

// Cloudflare's published IPv4 ranges. A domain whose records are all in here is proxied through Cloudflare, so the
// proxy (not this server) is what DNS shows; the request still reaches this server.
var cloudflareRanges = mustCIDRs(
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18",
	"190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
)

func mustCIDRs(list ...string) []*net.IPNet {
	var out []*net.IPNet
	for _, c := range list {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

const (
	dnsNone       = "none"       // no address records yet
	dnsHere       = "here"       // points at this server
	dnsCloudflare = "cloudflare" // proxied through Cloudflare
	dnsElsewhere  = "elsewhere"  // points somewhere else
)

// classifyDNS says where a domain's addresses point. here is the set of this server's own addresses.
func classifyDNS(ips []string, here map[string]bool) string {
	if len(ips) == 0 {
		return dnsNone
	}
	allCF := true
	for _, s := range ips {
		ip := net.ParseIP(s)
		if ip == nil {
			continue
		}
		if here[s] {
			return dnsHere
		}
		inCF := false
		for _, n := range cloudflareRanges {
			if n.Contains(ip) {
				inCF = true
				break
			}
		}
		allCF = allCF && inCF
	}
	if allCF {
		return dnsCloudflare
	}
	return dnsElsewhere
}

func serverAddresses() map[string]bool {
	here := map[string]bool{}
	if ip, err := serverPublicIP(); err == nil && ip != "" {
		here[ip] = true
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				here[ipn.IP.String()] = true
			}
		}
	}
	return here
}

func lookupIPv4(domain string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		if a.IP.To4() != nil {
			out = append(out, a.IP.String())
		}
	}
	return out
}

// ---- the work ----

type certDeps struct {
	now    func() time.Time
	exists func(domain string) bool
	lookup func(domain string) []string
	here   func() map[string]bool
	issue  func(domain string, www bool) (string, error)
}

func realCertDeps() certDeps {
	return certDeps{
		now:    time.Now,
		exists: func(d string) bool { _, err := runRootHelper(nil, "cert-exists", d); return err == nil },
		lookup: lookupIPv4,
		here:   serverAddresses,
		issue: func(d string, www bool) (string, error) {
			args := []string{"cert-issue", d}
			if www {
				args = append(args, "--www")
			}
			if e := acmeEmail(); e != "" {
				args = append(args, "--email", e)
			}
			out, err := runRootHelper(nil, args...)
			return string(out), err
		},
	}
}

// failureBackoff: how long to wait after the Nth real failure (nil = stop trying).
func failureBackoff(failed int) (time.Duration, bool) {
	switch failed {
	case 1:
		return 5 * time.Minute, true
	case 2:
		return 30 * time.Minute, true
	case 3:
		return 2 * time.Hour, true
	}
	return 0, false
}

// dnsBackoff: how long to wait before checking DNS again, by how long the job has been waiting.
func dnsBackoff(waited time.Duration) time.Duration {
	switch {
	case waited < 2*time.Minute:
		return 30 * time.Second
	case waited < 10*time.Minute:
		return time.Minute
	case waited < time.Hour:
		return 5 * time.Minute
	}
	return 15 * time.Minute
}

// shortReason turns certbot's long output into a sentence a person can act on.
func shortReason(out string) string {
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "too many") || strings.Contains(low, "rate limit"):
		return "Let's Encrypt says there have been too many attempts for this domain; DockLite will try again later."
	case strings.Contains(low, "timeout") || strings.Contains(low, "connection refused") || strings.Contains(low, "could not connect"):
		return "Let's Encrypt couldn't reach this server on port 80. Check the firewall (ports 80 and 443) and, if the domain is on Cloudflare, that it points here."
	case strings.Contains(low, "unauthorized") || strings.Contains(low, "404") || strings.Contains(low, "invalid response"):
		return "Let's Encrypt reached a server but not this site. The domain's DNS probably points somewhere else."
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			if len(l) > 220 {
				l = l[:220]
			}
			return l
		}
	}
	return "certbot failed"
}

// processCertJob does one attempt for one job and records the outcome.
func (q *certQueue) processCertJob(j certJob, d certDeps) {
	now := d.now()
	finish := func(state, msg string, next time.Time, failed int) {
		q.update(j.Domain, func(x *certJob) { x.State, x.Message, x.NextAttempt, x.FailedAttempts = state, msg, next, failed })
		auditSystem("ssl.auto", j.Domain, map[string]any{"state": state, "message": msg})
	}
	if d.exists(j.Domain) {
		finish(certIssued, "This site already has a certificate.", time.Time{}, 0)
		return
	}
	here := d.here()
	main := classifyDNS(d.lookup(j.Domain), here)
	if main == dnsNone || main == dnsElsewhere {
		if now.Sub(j.CreatedAt) > dnsWaitLimit {
			finish(certFailed, "Gave up after 48 hours: the domain's DNS never pointed at this server. Fix the DNS record, then press Try now.", time.Time{}, j.FailedAttempts)
			return
		}
		why := "has no DNS record yet"
		if main == dnsElsewhere {
			why = "points to a different server"
		}
		q.update(j.Domain, func(x *certJob) {
			x.Message = fmt.Sprintf("Waiting for DNS: %s %s. DockLite checks again by itself and gets the certificate as soon as it points here.", j.Domain, why)
			x.NextAttempt = now.Add(dnsBackoff(now.Sub(j.CreatedAt)))
		})
		return
	}
	www := false
	note := ""
	if j.IncludeWww && !strings.HasPrefix(j.Domain, "www.") {
		if c := classifyDNS(d.lookup("www."+j.Domain), here); c == dnsHere || c == dnsCloudflare {
			www = true
		} else {
			note = " (www." + j.Domain + " isn't pointed here, so it's not included)"
		}
	}
	q.update(j.Domain, func(x *certJob) { x.State, x.Message = certIssuing, "Asking Let's Encrypt for the certificate…" })
	out, err := d.issue(j.Domain, www)
	if err == nil {
		finish(certIssued, "HTTPS certificate issued."+note, time.Time{}, 0)
		return
	}
	failed := j.FailedAttempts + 1
	reason := shortReason(out)
	if wait, again := failureBackoff(failed); again {
		finish(certWaiting, fmt.Sprintf("%s (attempt %d; trying again in %s)", reason, failed, wait.Round(time.Minute)), now.Add(wait), failed)
		return
	}
	finish(certFailed, reason+" DockLite stopped trying; fix the cause, then press Try now.", time.Time{}, failed)
}

// RunAutoCerts works through due jobs for the life of the agent.
func (h *Handlers) RunAutoCerts(ctx context.Context) {
	if demo.On {
		return
	}
	// A job that was mid-attempt when the agent stopped should just try again.
	autoCerts.mu.Lock()
	for _, j := range autoCerts.jobs {
		if j.State == certIssuing {
			j.State, j.NextAttempt = certWaiting, time.Now().Add(10*time.Second)
		}
	}
	autoCerts.saveLocked()
	autoCerts.mu.Unlock()

	deps := realCertDeps()
	tick := time.NewTicker(certTickInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if j, ok := autoCerts.next(time.Now()); ok {
				autoCerts.processCertJob(j, deps)
			}
		}
	}
}

// EnqueueCert starts automatic HTTPS for a new site (first check in 20 seconds, so the nginx entry is live).
func (h *Handlers) EnqueueCert(domain string, includeWww bool) certJob {
	return autoCerts.enqueue(strings.ToLower(domain), includeWww, 20*time.Second)
}

// ---- settings ----

func readSettings() map[string]string {
	m := map[string]string{}
	if data, err := os.ReadFile(settingsFile); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

// acmeEmail is the contact address for Let's Encrypt (expiry notices): DOCKLITE_ACME_EMAIL, or the saved setting.
func acmeEmail() string {
	if v := strings.TrimSpace(os.Getenv("DOCKLITE_ACME_EMAIL")); v != "" {
		return v
	}
	return strings.TrimSpace(readSettings()["acme_email"])
}

// ---- API ----

// AutoCerts: GET lists automatic-certificate jobs; POST {"domain": "x.com"} (re)starts one; POST {"all": true} starts one for
// every site that has no certificate yet. Admin only.
func (h *Handlers) AutoCerts(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"jobs": autoCerts.snapshot(), "email": acmeEmail()})
	case http.MethodPost:
		var body struct {
			Domain string  `json:"domain"`
			All    bool    `json:"all"`
			Email  *string `json:"email"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Email != nil {
			e := strings.TrimSpace(*body.Email)
			if e != "" && (!strings.Contains(e, "@") || strings.ContainsAny(e, " ;\n\t")) {
				writeError(w, http.StatusBadRequest, "enter a valid email address")
				return
			}
			s := readSettings()
			s["acme_email"] = e
			if data, err := json.Marshal(s); err == nil {
				_ = os.MkdirAll(filepath.Dir(settingsFile), 0o750)
				_ = os.WriteFile(settingsFile, data, 0o640)
			}
			writeJSON(w, http.StatusOK, map[string]any{"email": e})
			return
		}
		started := []string{}
		if body.All {
			sites, err := h.store.ListSites()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			for _, s := range sites {
				if !isValidDomain(s.Domain) {
					continue
				}
				if _, err := runRootHelper(nil, "cert-exists", s.Domain); err == nil {
					continue
				}
				h.EnqueueCert(s.Domain, true)
				started = append(started, s.Domain)
			}
		} else {
			d := strings.ToLower(strings.TrimSpace(body.Domain))
			if !isValidDomain(d) {
				writeError(w, http.StatusBadRequest, "enter a valid domain")
				return
			}
			h.EnqueueCert(d, !strings.HasPrefix(d, "www."))
			started = append(started, d)
		}
		h.audit(r, "ssl.auto", strings.Join(started, ","), map[string]any{"count": len(started)})
		writeJSON(w, http.StatusOK, map[string]any{"started": started, "jobs": autoCerts.snapshot()})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
