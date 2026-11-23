package main

import (
	"encoding/json"
	"net/url"
)

func init() {
	register(command{
		Path: []string{"nginx", "enable"}, Summary: "Switch a site's nginx file back on (checked; undone if nginx rejects it)",
		Usage: "nginx enable <domain>", AdminOnly: true, Destructive: true,
		Examples: []string{"docklite nginx enable example.com --yes"},
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite nginx enable <domain>")
			}
			if err := a.confirm("switch on nginx for " + args[0] + " (visitors will reach it)"); err != nil {
				return err
			}
			data, err := a.post("/api/nginx/sites/"+url.PathEscape(args[0])+"/enable", nil)
			if err != nil {
				return err
			}
			a.emitValue(json.RawMessage(data), func() { a.say("nginx is on for %s", args[0]) })
			return nil
		},
	})
}

// disabledButRunning names the sites whose container is running but whose nginx file is switched off or missing,
// so visitors never reach them. sites is the nginx listing, running the domains of running site containers.
func disabledButRunning(sites []nginxSiteRow, running map[string]bool) (off, noFile []string) {
	for _, s := range sites {
		if !running[s.Domain] || s.Enabled {
			continue
		}
		if s.HasConfig {
			off = append(off, s.Domain)
		} else {
			noFile = append(noFile, s.Domain)
		}
	}
	return off, noFile
}

type nginxSiteRow struct {
	Domain    string `json:"domain"`
	Enabled   bool   `json:"enabled"`
	HasConfig bool   `json:"hasConfig"`
}
