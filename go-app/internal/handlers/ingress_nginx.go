package handlers

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// What the Ingress tab shows when nginx is the reverse proxy: read from nginx's own site configs, so there is nothing
// to guess ("unknown") about.

type ingressSites struct {
	Sites       int `json:"sites"`       // real server blocks (the catch-all default doesn't count)
	HTTPS       int `json:"https"`       // listening on 443
	Redirecting int `json:"redirecting"` // answering on 80 with a redirect to https
	HSTS        int `json:"hsts"`        // sending a Strict-Transport-Security header
}

var (
	reListen443  = regexp.MustCompile(`(?m)^\s*listen\s+[^;]*\b443\b`)
	reRedirect   = regexp.MustCompile(`return\s+30[1278]\s+https://`)
	reServerName = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
)

// serverBlocks splits an nginx config into the text of each top-level `server { ... }`.
func serverBlocks(conf string) []string {
	var out []string
	depth, start := 0, -1
	for i := 0; i < len(conf); i++ {
		switch conf[i] {
		case '{':
			if depth == 0 {
				// Is this brace the one that opens a server block? Look back for the word "server".
				head := strings.TrimRight(conf[:i], " \t\r\n")
				if strings.HasSuffix(head, "server") {
					start = len(head) - len("server")
				}
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, conf[start:i+1])
				start = -1
			}
			if depth < 0 {
				depth = 0
			}
		}
	}
	return out
}

// summarizeIngress counts what the server blocks in the given configs do. Blocks that only name `_` (the catch-all) are skipped.
func summarizeIngress(confs []string) ingressSites {
	var s ingressSites
	for _, conf := range confs {
		for _, block := range serverBlocks(conf) {
			m := reServerName.FindStringSubmatch(block)
			if m == nil || strings.TrimSpace(m[1]) == "_" {
				continue
			}
			s.Sites++
			if reListen443.MatchString(block) {
				s.HTTPS++
				if strings.Contains(block, "Strict-Transport-Security") {
					s.HSTS++
				}
			}
			if reRedirect.MatchString(block) {
				s.Redirecting++
			}
		}
	}
	return s
}

// readEnabledSites reads every file nginx has enabled (symlinks followed).
func readEnabledSites(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, string(data))
	}
	return out
}

// describeCount is "all 13", "11 of 13" or "none", for the cards.
func describeCount(n, of int) string {
	switch {
	case of == 0:
		return "no sites yet"
	case n == 0:
		return "none"
	case n >= of:
		return fmt.Sprintf("all %d", of)
	}
	return fmt.Sprintf("%d of %d", n, of)
}

// listeningPorts reports which of the given local ports accept connections.
func listeningPorts(ports ...int) []int {
	var up []int
	for _, p := range ports {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", p), 400*time.Millisecond)
		if err == nil {
			c.Close()
			up = append(up, p)
		}
	}
	sort.Ints(up)
	return up
}
