package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

// Several servers in one dashboard. Every server runs its own DockLite agent (the GUI and terminal app are optional on
// the others). One server is the hub: it keeps the other servers' addresses and the tokens they made for it, and passes
// requests on. A server shares itself with a join string ("dlj1." + base64 of {url, token, name}) that this file makes
// (`POST /api/servers/join-code`, `docklite share`) and reads (`POST /api/servers`).

const joinPrefix = "dlj1."

type joinCode struct {
	URL   string `json:"u"`
	Token string `json:"t"`
	Name  string `json:"n"`
}

func encodeJoin(j joinCode) string {
	b, _ := json.Marshal(j)
	return joinPrefix + base64.RawURLEncoding.EncodeToString(b)
}

func decodeJoin(s string) (joinCode, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, joinPrefix) {
		return joinCode{}, errors.New("that isn't a join code; run `docklite share` on the other server to get one")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, joinPrefix))
	if err != nil {
		return joinCode{}, errors.New("the join code is damaged; copy it again, all of it")
	}
	var j joinCode
	if json.Unmarshal(raw, &j) != nil || j.URL == "" || j.Token == "" {
		return joinCode{}, errors.New("the join code is incomplete; copy it again, all of it")
	}
	return j, nil
}

// validServerURL accepts http(s) addresses only, and refuses link-local ones (cloud metadata services).
func validServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("the server's address must look like https://smoll.docklite.net or http://203.0.113.7:3000")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return "", errors.New("that address can't be a DockLite server")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/"), nil
}

// ServerJoinCode (POST /api/servers/join-code, admins): make a token for a hub and wrap it with this server's address.
func (h *Handlers) ServerJoinCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = readJSON(w, r, &body)
	st := h.panelDomainState()
	secret, err := generateTokenSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	hash, fp, err := hashToken(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash token")
		return
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		label = "a hub"
	}
	role := "admin"
	issued := "multi-server hub"
	rec := store.TokenRecord{Name: "hub: " + label, TokenHash: hash, TokenFingerprint: fp, Role: &role, IssuedFor: &issued}
	if uid, ok := readUserIDFromContext(r); ok {
		rec.UserID = &uid
	}
	// No expiry: a hub whose token ran out would just stop working. It is revocable from the token list.
	created, err := h.store.CreateToken(rec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "server.join-code", label, map[string]any{"tokenId": created.ID})
	name, _ := os.Hostname()
	out := map[string]any{"join": encodeJoin(joinCode{URL: st.URL, Token: secret, Name: name}), "url": st.URL, "name": name, "https": strings.HasPrefix(st.URL, "https://")}
	if !strings.HasPrefix(st.URL, "https://") {
		out["warning"] = "This server's address is plain http, so the token would travel unencrypted. Set a domain with HTTPS first (Settings → Linked domain) if the servers are not on a private network."
	}
	writeJSON(w, http.StatusOK, out)
}

// Servers: GET lists connected servers; POST adds one from a join code (or name + url + token).
func (h *Handlers) Servers(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := h.store.ListRemoteServers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"servers": list})
	case http.MethodPost:
		h.addServer(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) addServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Join  string `json:"join"`
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name, rawURL, token := strings.TrimSpace(body.Name), body.URL, strings.TrimSpace(body.Token)
	if strings.TrimSpace(body.Join) != "" {
		j, err := decodeJoin(body.Join)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rawURL, token = j.URL, j.Token
		if name == "" {
			name = j.Name
		}
	}
	u, err := validServerURL(rawURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if name == "" || len(name) > 40 || token == "" {
		writeError(w, http.StatusBadRequest, "give the server a name (up to 40 characters) and a join code")
		return
	}
	if err := checkRemote(r.Context(), u, token); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	rec, err := h.store.AddRemoteServer(name, u, token)
	if errors.Is(err, store.ErrServerNameTaken) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "server.add", rec.Name, map[string]any{"url": rec.URL})
	writeJSON(w, http.StatusOK, map[string]any{"server": rec})
}

var remoteClient = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// checkRemote asks the other server for its summary: proves the address works and the token is accepted.
func checkRemote(ctx context.Context, base, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := remoteClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach %s: %v", base, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return errors.New("that server didn't accept the token; make a new join code with `docklite share` there")
	case resp.StatusCode >= 300:
		return fmt.Errorf("that address answered with %d, so it doesn't look like a DockLite server", resp.StatusCode)
	}
	return nil
}

var (
	serverRouteRe = regexp.MustCompile(`^/api/servers/(\d+)(?:/(proxy)(/.*)?)?$`)
	// What a hub may read from another server, and the few actions it may start there.
	proxyReadRe   = regexp.MustCompile(`^/(summary|health|containers(/all|/[A-Za-z0-9]+/(logs|stats))?|databases(/stats)?|server/(overview|stats|services|storage|security|updates|ports)|network/(overview|firewall|ingress|public-ip)|ssl/status|panel-domain|files)$`)
	proxyActionRe = regexp.MustCompile(`^/containers/[A-Za-z0-9]+/(start|stop|restart)$`)
)

// ServerRoutes handles /api/servers/{id} (DELETE) and /api/servers/{id}/proxy/<path>.
func (h *Handlers) ServerRoutes(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	m := serverRouteRe.FindStringSubmatch(r.URL.Path)
	if m == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	srv, err := h.store.GetRemoteServer(id)
	if err != nil || srv == nil {
		writeError(w, http.StatusNotFound, "no such server")
		return
	}
	if m[2] == "" {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		_ = h.store.DeleteRemoteServer(id)
		h.audit(r, "server.remove", srv.Name, nil)
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}
	path := m[3]
	allowed := (r.Method == http.MethodGet && proxyReadRe.MatchString(path)) || (r.Method == http.MethodPost && proxyActionRe.MatchString(path))
	if !allowed {
		writeError(w, http.StatusForbidden, "the hub doesn't pass that request on to other servers")
		return
	}
	target := srv.URL + "/api" + path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, _ := http.NewRequestWithContext(r.Context(), r.Method, target, nil)
	req.Header.Set("Authorization", "Bearer "+srv.Token)
	resp, err := remoteClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("couldn't reach %s: %v", srv.Name, err))
		return
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 20<<20))
}
