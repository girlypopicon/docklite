package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func init() {
	// ---- SSL ----
	register(command{
		Path: []string{"ssl", "status"}, Summary: "Certificates on this server and when they expire",
		Usage: "ssl status",
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/ssl/status")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				AllCerts []struct {
					Domain          string `json:"domain"`
					Status          string `json:"status"`
					DaysUntilExpiry *int   `json:"daysUntilExpiry"`
				} `json:"allCerts"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				a.emit(data)
				return nil
			}
			rows := [][]string{}
			for _, c := range resp.AllCerts {
				days := "-"
				if c.DaysUntilExpiry != nil {
					days = strconv.Itoa(*c.DaysUntilExpiry)
				}
				rows = append(rows, []string{c.Domain, c.Status, days})
			}
			if len(rows) == 0 {
				fmt.Fprintln(a.out, "no certificates found")
				return nil
			}
			a.table([]string{"DOMAIN", "STATUS", "DAYS LEFT"}, rows)
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "issue"}, Summary: "Get a Let's Encrypt certificate for a domain (its DNS must point at this server)",
		Usage: "ssl issue <domain> [--www] [--email ADDRESS]", AdminOnly: true,
		Examples: []string{"docklite ssl issue example.com --www --email me@example.com"},
		Run: func(a *app, args []string) error {
			var www bool
			var email string
			pos, err := a.flags("ssl issue", args, func(fs *flag.FlagSet) {
				fs.BoolVar(&www, "www", false, "")
				fs.StringVar(&email, "email", "", "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite ssl issue <domain> [--www] [--email ADDRESS]")
			}
			payload := map[string]any{"domain": pos[0], "includeWww": www}
			if email != "" {
				payload["email"] = email
			}
			data, err := a.post("/api/ssl/issue", payload)
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { a.say("certificate issued for %s", pos[0]) })
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "renew"}, Summary: "Renew a certificate now", Usage: "ssl renew <domain>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite ssl renew <domain>")
			}
			data, err := a.post("/api/ssl/renew", map[string]any{"domain": args[0]})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { a.say("certificate for %s renewed", args[0]) })
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "delete"}, Summary: "Delete a certificate", Usage: "ssl delete <domain>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite ssl delete <domain>")
			}
			if err := a.confirm("delete the certificate for " + args[0]); err != nil {
				return err
			}
			data, err := a.post("/api/ssl/delete", map[string]any{"domain": args[0]})
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { a.say("certificate for %s deleted", args[0]) })
			return nil
		},
	})

	// ---- nginx ----
	register(command{
		Path: []string{"nginx", "sites"}, Summary: "DockLite-managed nginx sites and whether each is enabled", Usage: "nginx sites", AdminOnly: true,
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/nginx/sites") },
	})
	register(command{
		Path: []string{"nginx", "show"}, Summary: "Print a site's nginx config", Usage: "nginx show <domain>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite nginx show <domain>")
			}
			data, err := a.get("/api/nginx/sites/" + url.PathEscape(args[0]))
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Config string `json:"config"`
			}
			if json.Unmarshal(data, &resp) == nil && resp.Config != "" {
				fmt.Fprint(a.out, resp.Config)
				return nil
			}
			a.emit(data)
			return nil
		},
	})
	register(command{
		Path: []string{"nginx", "test"}, Summary: "Check that nginx's configuration is valid", Usage: "nginx test", AdminOnly: true,
		Run: func(a *app, args []string) error { return nginxAction(a, "test") },
	})
	register(command{
		Path: []string{"nginx", "reload"}, Summary: "Reload nginx (tests the config first)", Usage: "nginx reload", AdminOnly: true,
		Run: func(a *app, args []string) error { return nginxAction(a, "reload") },
	})

	// ---- DNS ----
	register(command{
		Path: []string{"dns", "zones"}, Summary: "Domains DockLite manages DNS for (Cloudflare)", Usage: "dns zones", AdminOnly: true,
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/dns/zones") },
	})
	register(command{
		Path: []string{"dns", "records"}, Summary: "DNS records for one domain", Usage: "dns records <domain|zone-id>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite dns records <domain|zone-id>")
			}
			zoneID := args[0]
			if _, err := strconv.Atoi(zoneID); err != nil {
				data, err := a.get("/api/dns/zones")
				if err != nil {
					return err
				}
				var resp struct {
					Zones []struct {
						ID     int    `json:"id"`
						Domain string `json:"domain"`
					} `json:"zones"`
				}
				_ = json.Unmarshal(data, &resp)
				zoneID = ""
				for _, z := range resp.Zones {
					if z.Domain == args[0] {
						zoneID = strconv.Itoa(z.ID)
					}
				}
				if zoneID == "" {
					return &exitError{code: exitNotFound, msg: fmt.Sprintf("no DNS zone for %q (see: docklite dns zones)", args[0])}
				}
			}
			return a.getAndEmit("/api/dns/records?zone_id=" + zoneID)
		},
	})

	// ---- server ----
	for _, item := range []struct{ name, path, summary string }{
		{"overview", "/api/server/overview", "Host health: CPU, memory, disk, load, uptime"},
		{"services", "/api/server/services", "Docker, DockLite, nginx, Traefik and what each is doing"},
		{"updates", "/api/server/updates", "Pending system updates"},
		{"storage", "/api/server/storage", "Disk and Docker storage use"},
		{"security", "/api/server/security", "Basic host security checks"},
	} {
		path := item.path
		register(command{
			Path: []string{"server", item.name}, Summary: item.summary, Usage: "server " + item.name, AdminOnly: true,
			Run: func(a *app, args []string) error { return a.getAndEmit(path) },
		})
	}
	register(command{
		Path: []string{"server", "logs"}, Summary: "Recent logs: system, docklite, proxy, traefik or legacy-api",
		Usage: "server logs <system|docklite|proxy|traefik|legacy-api> [--tail N]", AdminOnly: true,
		Examples: []string{"docklite server logs docklite --tail 200"},
		Run: func(a *app, args []string) error {
			tail := 200
			pos, err := a.flags("server logs", args, func(fs *flag.FlagSet) { fs.IntVar(&tail, "tail", 200, "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite server logs <system|docklite|proxy|traefik|legacy-api> [--tail N]")
			}
			data, err := a.get("/api/server/logs?target=" + url.QueryEscape(pos[0]) + "&tail=" + strconv.Itoa(tail))
			if err != nil {
				return err
			}
			var resp struct {
				Logs string `json:"logs"`
			}
			if json.Unmarshal(data, &resp) != nil || a.opts.JSON {
				a.emit(data)
				return nil
			}
			fmt.Fprint(a.out, resp.Logs)
			return nil
		},
	})
	register(command{
		Path: []string{"server", "service"}, Summary: "Start, stop, restart or reload a service (traefik, proxy, legacy-api, docklite)",
		Usage: "server service <traefik|proxy|legacy-api|docklite> <start|stop|restart|reload>", AdminOnly: true, Destructive: true,
		Examples: []string{"docklite server service traefik stop --yes"},
		Run: func(a *app, args []string) error {
			if len(args) != 2 {
				return usageError("usage: docklite server service <traefik|proxy|legacy-api|docklite> <start|stop|restart|reload>")
			}
			service, action := args[0], args[1]
			if action == "stop" || action == "restart" {
				if err := a.confirm(fmt.Sprintf("%s %s", action, service)); err != nil {
					return err
				}
			}
			if _, err := a.post("/api/server/services/action", map[string]any{"service": service, "action": action}); err != nil {
				return err
			}
			a.emitValue(map[string]any{"service": service, "action": action, "ok": true}, func() { a.say("%s: %s done", service, action) })
			return nil
		},
	})

	// ---- databases ----
	register(command{
		Path: []string{"databases", "list"}, Summary: "Database containers DockLite manages", Usage: "databases list",
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/databases") },
	})

	// ---- backups ----
	register(command{
		Path: []string{"backups", "list"}, Summary: "Backups that exist, and whether each succeeded", Usage: "backups list", AdminOnly: true,
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/backups") },
	})
	register(command{
		Path: []string{"backups", "delete"}, Summary: "Delete a backup and its file", Usage: "backups delete <id>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite backups delete <id>")
			}
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return usageError("backup id must be a number (see: docklite backups list)")
			}
			if err := a.confirm(fmt.Sprintf("delete backup %d", id)); err != nil {
				return err
			}
			if _, err := a.del("/api/backups?id=" + strconv.Itoa(id)); err != nil {
				return err
			}
			a.say("backup %d deleted", id)
			return nil
		},
	})

	// ---- users ----
	register(command{
		Path: []string{"users", "list"}, Summary: "DockLite accounts", Usage: "users list", AdminOnly: true,
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/users") },
	})
	register(command{
		Path: []string{"users", "create"}, Summary: "Create a DockLite account (password read from the terminal or stdin)",
		Usage: "users create <username> [--admin] [--password-stdin]", AdminOnly: true,
		Examples: []string{"echo \"$PW\" | docklite users create alice --password-stdin"},
		Run: func(a *app, args []string) error {
			var admin, stdin bool
			pos, err := a.flags("users create", args, func(fs *flag.FlagSet) {
				fs.BoolVar(&admin, "admin", false, "")
				fs.BoolVar(&stdin, "password-stdin", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite users create <username> [--admin] [--password-stdin]")
			}
			password, err := a.newPassword(stdin)
			if err != nil {
				return err
			}
			if _, err := a.post("/api/users", map[string]any{"username": pos[0], "password": password, "isAdmin": admin}); err != nil {
				return err
			}
			a.emitValue(map[string]any{"username": pos[0], "admin": admin, "created": true}, func() { a.say("user %s created", pos[0]) })
			return nil
		},
	})
	register(command{
		Path: []string{"users", "passwd"}, Summary: "Set a user's password (admin) — id from `users list`",
		Usage: "users passwd <id> [--password-stdin]", AdminOnly: true,
		Run: func(a *app, args []string) error {
			var stdin bool
			pos, err := a.flags("users passwd", args, func(fs *flag.FlagSet) { fs.BoolVar(&stdin, "password-stdin", false, "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite users passwd <id> [--password-stdin]")
			}
			id, err := strconv.Atoi(pos[0])
			if err != nil || id <= 0 {
				return usageError("user id must be a number (see: docklite users list)")
			}
			password, err := a.newPassword(stdin)
			if err != nil {
				return err
			}
			if _, err := a.post("/api/users/password", map[string]any{"userId": id, "newPassword": password}); err != nil {
				return err
			}
			a.say("password changed for user %d", id)
			return nil
		},
	})
	register(command{
		Path: []string{"users", "delete"}, Summary: "Delete a user (their sites move to you)", Usage: "users delete <id>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite users delete <id>")
			}
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return usageError("user id must be a number (see: docklite users list)")
			}
			if err := a.confirm(fmt.Sprintf("delete user %d", id)); err != nil {
				return err
			}
			if _, err := a.del("/api/users?id=" + strconv.Itoa(id)); err != nil {
				return err
			}
			a.say("user %d deleted", id)
			return nil
		},
	})
}

func nginxAction(a *app, action string) error {
	data, err := a.post("/api/nginx/"+action, nil)
	if err != nil {
		return err
	}
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &resp)
	if a.opts.JSON {
		a.emit(data)
	}
	if !resp.OK {
		return &exitError{code: exitFailure, msg: "nginx " + action + " failed: " + strings.TrimSpace(resp.Error)}
	}
	a.say("nginx %s: ok", action)
	return nil
}

// newPassword reads a new password: from a line on stdin (for scripts) or
// twice from the terminal.
func (a *app) newPassword(fromStdin bool) (string, error) {
	if fromStdin {
		return a.readLine("")
	}
	first, err := a.readSecret("New password: ")
	if err != nil {
		return "", err
	}
	second, err := a.readSecret("Repeat it: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", usageError("the two passwords don't match")
	}
	return first, nil
}
