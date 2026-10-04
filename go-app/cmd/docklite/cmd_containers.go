package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// containerRow is one entry of /api/containers/all.
type containerRow struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Image   string            `json:"image"`
	Ports   string            `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Owner   string            `json:"owner_username"`
	Domain  string            `json:"domain"`
	Tracked bool              `json:"tracked"`
}

func (c containerRow) kind() string {
	switch c.Labels["docklite.type"] {
	case "static", "php", "node":
		return "site"
	case "postgres":
		return "database"
	}
	if c.Labels["docklite.database"] != "" {
		return "database"
	}
	return "other"
}

func (c containerRow) domain() string {
	if c.Domain != "" {
		return c.Domain
	}
	return c.Labels["docklite.domain"]
}

func (c containerRow) running() bool { return c.State == "running" }

func (c containerRow) shortID() string {
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

// shortImage trims an image id like sha256:b0f7830b6bfa... to what docker shows.
func shortImage(image string) string {
	if strings.HasPrefix(image, "sha256:") && len(image) > 19 {
		return image[7:19]
	}
	return image
}

// dedupePorts drops repeats: Docker lists the same mapping for IPv4 and IPv6.
func dedupePorts(ports string) string {
	if ports == "" {
		return "-"
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range strings.Split(ports, ",") {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}

func (a *app) listContainers() ([]containerRow, error) {
	data, err := a.get("/api/containers/all")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Containers []containerRow `json:"containers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unexpected response from the server: %w", err)
	}
	// Same order as the web UI: running first, then sites, databases, other.
	rank := map[string]int{"site": 0, "database": 1, "other": 2}
	sort.SliceStable(resp.Containers, func(i, j int) bool {
		x, y := resp.Containers[i], resp.Containers[j]
		if x.running() != y.running() {
			return x.running()
		}
		if rank[x.kind()] != rank[y.kind()] {
			return rank[x.kind()] < rank[y.kind()]
		}
		return x.Name < y.Name
	})
	return resp.Containers, nil
}

// resolveContainer turns what a person types — an id, an id prefix, a
// container name or a site's domain — into exactly one container.
func (a *app) resolveContainer(ref string) (containerRow, error) {
	containers, err := a.listContainers()
	if err != nil {
		return containerRow{}, err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return containerRow{}, usageError("which container? give a name, domain or id (see: docklite containers list)")
	}
	pick := func(match func(containerRow) bool) []containerRow {
		var out []containerRow
		for _, c := range containers {
			if match(c) {
				out = append(out, c)
			}
		}
		return out
	}
	for _, match := range []func(containerRow) bool{
		func(c containerRow) bool { return c.ID == ref },
		func(c containerRow) bool { return c.Name == ref },
		func(c containerRow) bool { return c.domain() != "" && c.domain() == ref },
		func(c containerRow) bool { return len(ref) >= 4 && strings.HasPrefix(c.ID, ref) },
		func(c containerRow) bool {
			return strings.Contains(c.Name, ref) || (c.domain() != "" && strings.Contains(c.domain(), ref))
		},
	} {
		found := pick(match)
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		}
		var names []string
		for _, c := range found {
			names = append(names, c.Name)
		}
		return containerRow{}, &exitError{code: exitUsage, msg: fmt.Sprintf("%q matches %d containers (%s) — be more specific", ref, len(found), strings.Join(names, ", "))}
	}
	return containerRow{}, &exitError{code: exitNotFound, msg: fmt.Sprintf("no container matches %q (see: docklite containers list)", ref)}
}

func init() {
	list := func(a *app, args []string) error {
		var kind string
		var runningOnly, stoppedOnly bool
		if _, err := a.flags("containers list", args, func(fs *flag.FlagSet) {
			fs.StringVar(&kind, "kind", "", "")
			fs.BoolVar(&runningOnly, "running", false, "")
			fs.BoolVar(&stoppedOnly, "stopped", false, "")
		}); err != nil {
			return err
		}
		if kind != "" && kind != "site" && kind != "database" && kind != "other" {
			return usageError("--kind must be site, database or other")
		}
		containers, err := a.listContainers()
		if err != nil {
			return err
		}
		var shown []containerRow
		for _, c := range containers {
			if kind != "" && c.kind() != kind {
				continue
			}
			if runningOnly && !c.running() {
				continue
			}
			if stoppedOnly && c.running() {
				continue
			}
			shown = append(shown, c)
		}
		a.emitValue(map[string]any{"containers": shown}, func() {
			if len(shown) == 0 {
				fmt.Fprintln(a.out, "no containers match")
				return
			}
			rows := make([][]string, 0, len(shown))
			for _, c := range shown {
				name := c.Name
				if d := c.domain(); d != "" {
					name = d + " (" + c.Name + ")"
				}
				rows = append(rows, []string{c.shortID(), name, c.kind(), c.State, shortImage(c.Image), dedupePorts(c.Ports)})
			}
			a.table([]string{"ID", "NAME", "KIND", "STATE", "IMAGE", "PORTS"}, rows)
		})
		return nil
	}
	register(command{
		Path: []string{"containers", "list"}, Summary: "List containers (running first, then sites, databases, other)",
		Usage:    "containers list [--kind site|database|other] [--running|--stopped]",
		Examples: []string{"docklite containers list", "docklite containers list --kind site --running --json"},
		Run:      list,
	})
	register(command{
		Path: []string{"list"}, Summary: "Same as containers list", Usage: "list", Run: list,
	})

	lifecycle := func(action string) func(a *app, args []string) error {
		return func(a *app, args []string) error {
			pos, err := a.flags("containers "+action, args, nil)
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite containers %s <name|domain|id>", action)
			}
			c, err := a.resolveContainer(pos[0])
			if err != nil {
				return err
			}
			if action == "stop" {
				if err := a.confirm(fmt.Sprintf("stop %s", c.Name)); err != nil {
					return err
				}
			}
			if _, err := a.post("/api/containers/"+c.ID+"/"+action, nil); err != nil {
				return err
			}
			a.emitValue(map[string]any{"container": c.Name, "action": action, "ok": true}, func() {
				a.say("%s: %s done", c.Name, action)
			})
			return nil
		}
	}
	for _, action := range []string{"start", "stop", "restart"} {
		register(command{
			Path: []string{"containers", action}, Summary: strings.ToUpper(action[:1]) + action[1:] + " a container",
			Usage: "containers " + action + " <name|domain|id>", Destructive: action == "stop",
			Examples: []string{"docklite containers " + action + " example.com"},
			Run:      lifecycle(action),
		})
	}

	register(command{
		Path: []string{"containers", "logs"}, Summary: "Show a container's recent log output",
		Usage:    "containers logs <name|domain|id> [--tail N]",
		Examples: []string{"docklite containers logs example.com --tail 100"},
		Run: func(a *app, args []string) error {
			tail := 100
			pos, err := a.flags("containers logs", args, func(fs *flag.FlagSet) { fs.IntVar(&tail, "tail", 100, "") })
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite containers logs <name|domain|id> [--tail N]")
			}
			c, err := a.resolveContainer(pos[0])
			if err != nil {
				return err
			}
			data, err := a.get("/api/containers/" + c.ID + "/logs?tail=" + strconv.Itoa(tail))
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
		Path: []string{"containers", "inspect"}, Summary: "Full details of a container (as JSON)",
		Usage: "containers inspect <name|domain|id>",
		Run: func(a *app, args []string) error {
			pos, err := a.flags("containers inspect", args, nil)
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite containers inspect <name|domain|id>")
			}
			c, err := a.resolveContainer(pos[0])
			if err != nil {
				return err
			}
			return a.getAndEmit("/api/containers/" + c.ID + "/inspect")
		},
	})
	register(command{
		Path: []string{"containers", "delete"}, Summary: "Delete a container (and a site's DockLite record). Site files are kept.",
		Usage: "containers delete <name|domain|id>", Destructive: true,
		Run: func(a *app, args []string) error {
			pos, err := a.flags("containers delete", args, nil)
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite containers delete <name|domain|id>")
			}
			c, err := a.resolveContainer(pos[0])
			if err != nil {
				return err
			}
			if err := a.confirm(fmt.Sprintf("delete container %s", c.Name)); err != nil {
				return err
			}
			if _, err := a.del("/api/containers/" + c.ID + "/delete"); err != nil {
				return err
			}
			a.emitValue(map[string]any{"container": c.Name, "deleted": true}, func() { a.say("%s deleted", c.Name) })
			return nil
		},
	})

	register(command{
		Path: []string{"sites", "list"}, Summary: "List websites with their domain, type and state",
		Usage: "sites list",
		Run: func(a *app, args []string) error {
			return list(a, append([]string{"--kind", "site"}, args...))
		},
	})
	register(command{
		Path: []string{"sites", "create"}, Summary: "Create a website (static, php or node) for a domain",
		Usage:    "sites create <domain> [--type static|php|node] [--port N] [--no-www] [--user ID]",
		Examples: []string{"docklite sites create example.com", "docklite sites create app.example.com --type node --port 3000"},
		Run: func(a *app, args []string) error {
			var siteType string
			var port, userID int
			var noWww bool
			pos, err := a.flags("sites create", args, func(fs *flag.FlagSet) {
				fs.StringVar(&siteType, "type", "static", "")
				fs.IntVar(&port, "port", 0, "")
				fs.BoolVar(&noWww, "no-www", false, "")
				fs.IntVar(&userID, "user", 0, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 1 {
				return usageError("usage: docklite sites create <domain> [--type static|php|node]")
			}
			if siteType != "static" && siteType != "php" && siteType != "node" {
				return usageError("--type must be static, php or node")
			}
			payload := map[string]any{"domain": pos[0], "template_type": siteType, "include_www": !noWww}
			if port > 0 {
				payload["port"] = port
			}
			if userID > 0 {
				payload["user_id"] = userID
			}
			data, err := a.post("/api/containers", payload)
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Warning string `json:"warning"`
			}
			_ = json.Unmarshal(data, &resp)
			a.say("site %s created (%s)", pos[0], siteType)
			if resp.Warning != "" {
				fmt.Fprintln(a.errOut, "warning: "+resp.Warning)
			}
			return nil
		},
	})
	_ = url.QueryEscape
}
