package main

import (
	"encoding/json"
	"strings"
)

func init() {
	register(command{
		Path:      []string{"sites", "permissions"},
		Summary:   "Make site folders writable by the docklite group (so you, or an assistant logged in, can edit site files)",
		Usage:     "sites permissions [domain]",
		AdminOnly: true,
		Examples: []string{
			"docklite sites permissions                 # every site",
			"docklite sites permissions glowrom.com     # just one",
		},
		Run: func(a *app, args []string) error {
			pos, err := a.flags("sites permissions", args, nil)
			if err != nil {
				return err
			}
			if len(pos) > 1 {
				return usageError("usage: docklite sites permissions [domain]")
			}
			payload := map[string]any{}
			if len(pos) == 1 {
				payload["domain"] = strings.ToLower(pos[0])
			}
			data, err := a.post("/api/sites/permissions", payload)
			if err != nil {
				return err
			}
			if a.opts.JSON {
				a.emit(data)
				return nil
			}
			var resp struct {
				Sites []struct {
					Domain  string `json:"domain"`
					Changed int    `json:"changed"`
					Skipped int    `json:"skipped"`
				} `json:"sites"`
				Changed int `json:"changed"`
				Skipped int `json:"skipped"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return err
			}
			rows := [][]string{}
			for _, s := range resp.Sites {
				note := "already fine"
				if s.Changed > 0 {
					note = "made group-writable"
				}
				if s.Skipped > 0 {
					note += " (some files are owned by root; fix with sudo chmod -R g+rwX)"
				}
				rows = append(rows, []string{s.Domain, itoa(s.Changed), note})
			}
			if len(rows) == 0 && len(pos) == 1 {
				return &exitError{code: exitNotFound, msg: "no site named " + pos[0] + " (or its folder is outside /var/www/sites)"}
			}
			a.table([]string{"SITE", "CHANGED", "RESULT"}, rows)
			a.say("\nOwners and other users' permissions are untouched; only the docklite group gained access.")
			return nil
		},
	})
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
