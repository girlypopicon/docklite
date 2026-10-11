package main

import (
	"encoding/json"
	"flag"
	"strings"
)

type certJobView struct {
	Domain         string `json:"domain"`
	State          string `json:"state"`
	Message        string `json:"message"`
	FailedAttempts int    `json:"failedAttempts"`
}

func printCertJobs(a *app, jobs []certJobView) {
	rows := [][]string{}
	for _, j := range jobs {
		rows = append(rows, []string{j.Domain, j.State, j.Message})
	}
	a.table([]string{"DOMAIN", "STATE", "DETAILS"}, rows)
}

func init() {
	register(command{
		Path: []string{"ssl", "pending"}, Summary: "Automatic HTTPS: which sites are getting (or waiting for) a certificate",
		Usage: "ssl pending", AdminOnly: true,
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/ssl/auto")
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Jobs  []certJobView `json:"jobs"`
				Email string        `json:"email"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			if len(resp.Jobs) == 0 {
				a.say("No automatic certificate jobs. New sites get one by themselves once their DNS points here.")
				return nil
			}
			printCertJobs(a, resp.Jobs)
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "retry"}, Summary: "Try to get a site's HTTPS certificate again right now (or for every site without one with --all)",
		Usage: "ssl retry <domain> | ssl retry --all", AdminOnly: true,
		Examples: []string{"docklite ssl retry glowrom.com", "docklite ssl retry --all"},
		Run: func(a *app, args []string) error {
			var all bool
			pos, err := a.flags("ssl retry", args, func(fs *flag.FlagSet) { fs.BoolVar(&all, "all", false, "") })
			if err != nil {
				return err
			}
			payload := map[string]any{}
			switch {
			case all && len(pos) == 0:
				payload["all"] = true
			case !all && len(pos) == 1:
				payload["domain"] = strings.ToLower(pos[0])
			default:
				return usageError("usage: docklite ssl retry <domain> | docklite ssl retry --all")
			}
			data, err := a.post("/api/ssl/auto", payload)
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Started []string      `json:"started"`
				Jobs    []certJobView `json:"jobs"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			if len(resp.Started) == 0 {
				a.say("Nothing to do: every site already has a certificate.")
				return nil
			}
			a.say("Started: %s. DockLite checks DNS and asks for the certificates in the background; see progress with: docklite ssl pending", strings.Join(resp.Started, ", "))
			return nil
		},
	})
	register(command{
		Path: []string{"ssl", "email"}, Summary: "Show or set the contact email Let's Encrypt uses for expiry notices",
		Usage: "ssl email [address]", AdminOnly: true,
		Run: func(a *app, args []string) error {
			pos, err := a.flags("ssl email", args, nil)
			if err != nil {
				return err
			}
			if len(pos) > 1 {
				return usageError("usage: docklite ssl email [address]")
			}
			if len(pos) == 1 {
				if _, err := a.post("/api/ssl/auto", map[string]any{"email": pos[0]}); err != nil {
					return err
				}
				a.say("Certificate contact email set to %s", pos[0])
				return nil
			}
			data, err := a.get("/api/ssl/auto")
			if err != nil {
				return err
			}
			var resp struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(data, &resp)
			if resp.Email == "" {
				a.say("No contact email set. Certificates still work; Let's Encrypt just can't email expiry warnings. Set one with: docklite ssl email you@example.com")
			} else {
				a.say("%s", resp.Email)
			}
			return nil
		},
	})
}
