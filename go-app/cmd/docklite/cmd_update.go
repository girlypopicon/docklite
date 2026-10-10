package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"
)

// How often `docklite update` asks the agent how the update is going. A variable so tests can be quick.
var updatePollInterval = 3 * time.Second

// How long to keep waiting. An update builds the program, so give it room.
var updateWaitLimit = 25 * time.Minute

type updateStatus struct {
	Version         string   `json:"version"`
	LatestVersion   string   `json:"latestVersion"`
	UpdateAvailable bool     `json:"updateAvailable"`
	Notes           string   `json:"notes"`
	CheckError      string   `json:"checkError"`
	UpdateRunning   bool     `json:"updateRunning"`
	State           string   `json:"state"`
	StateTag        string   `json:"stateTag"`
	StateMessage    string   `json:"stateMessage"`
	Log             []string `json:"log"`
}

func (a *app) updateStatus(refresh bool) (*updateStatus, error) {
	path := "/api/system/update/status"
	if refresh {
		path += "?refresh=1"
	}
	data, err := a.get(path)
	if err != nil {
		return nil, err
	}
	var st updateStatus
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func init() {
	register(command{
		Path:        []string{"update"},
		Summary:     "Check for a newer DockLite and install it (your sites keep running)",
		Usage:       "update [--check] [--force] [--yes]",
		AdminOnly:   true,
		Destructive: true,
		Examples: []string{
			"docklite update --check        # just say whether a newer version exists",
			"docklite update --yes          # install the newest release and wait until it is running",
		},
		Run: func(a *app, args []string) error {
			var check, force bool
			pos, err := a.flags("update", args, func(fs *flag.FlagSet) {
				fs.BoolVar(&check, "check", false, "")
				fs.BoolVar(&force, "force", false, "")
			})
			if err != nil {
				return err
			}
			if len(pos) != 0 {
				return usageError("usage: docklite update [--check] [--force] [--yes]")
			}
			st, err := a.updateStatus(true)
			if err != nil {
				return err
			}
			if st.CheckError != "" && st.LatestVersion == "" {
				return &exitError{code: exitFailure, msg: "couldn't check for updates: " + st.CheckError}
			}
			if st.UpdateRunning {
				return &exitError{code: exitFailure, msg: "an update is already running; see its progress with: docklite update --check"}
			}
			if check || (!st.UpdateAvailable && !force) {
				a.emitValue(st, func() {
					a.say("installed: %s", st.Version)
					a.say("newest:    %s", st.LatestVersion)
					if st.UpdateAvailable {
						a.say("\nA newer version is available. Install it with: docklite update --yes")
						if n := strings.TrimSpace(st.Notes); n != "" {
							a.say("\nWhat's new:\n%s", n)
						}
					} else {
						a.say("\nYou have the newest version.")
					}
				})
				return nil
			}
			if err := a.confirm(fmt.Sprintf("update DockLite from %s to %s (sites keep running; the dashboard restarts for about a minute)", st.Version, st.LatestVersion)); err != nil {
				return err
			}
			payload := map[string]any{}
			if force {
				payload["force"] = true
			}
			if _, err := a.post("/api/system/update/run", payload); err != nil {
				return err
			}
			a.say("Update to %s started. Waiting for it to finish...", st.LatestVersion)
			return a.waitForUpdate(st.LatestVersion)
		},
	})
}

// waitForUpdate follows an update to its end. DockLite restarts partway through, so errors while it is
// down are expected and simply mean "keep waiting".
func (a *app) waitForUpdate(target string) error {
	deadline := time.Now().Add(updateWaitLimit)
	printed := 0
	var lastLine string
	down := false
	for time.Now().Before(deadline) {
		time.Sleep(updatePollInterval)
		st, err := a.updateStatus(false)
		if err != nil {
			if !down {
				a.say("(DockLite is restarting; waiting for it to come back)")
				down = true
			}
			continue
		}
		down = false
		// print only the lines we haven't shown yet (the agent sends the tail of the log)
		start := 0
		if lastLine != "" {
			for i := len(st.Log) - 1; i >= 0; i-- {
				if st.Log[i] == lastLine {
					start = i + 1
					break
				}
			}
		}
		for _, line := range st.Log[start:] {
			a.say("  %s", line)
			lastLine = line
			printed++
		}
		switch st.State {
		case "success":
			if strings.TrimPrefix(st.Version, "v") == strings.TrimPrefix(target, "v") {
				a.emitValue(st, func() { a.say("\nUpdated: DockLite %s is running.", st.Version) })
				return nil
			}
		case "failed", "rolled-back":
			msg := st.StateMessage
			if st.State == "rolled-back" {
				msg = "the update did not work, so DockLite went back to the version that did: " + msg
			}
			return &exitError{code: exitFailure, msg: msg}
		}
	}
	return &exitError{code: exitFailure, msg: "timed out waiting for the update; check its log with: docklite update --check"}
}
