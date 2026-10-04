package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"docklite-agent/internal/cli"
)

//go:embed docs/CLI.md
var cliGuide string

func init() {
	register(command{
		Path: []string{"version"}, Summary: "Print the CLI version",
		Usage: "version", NoClient: true,
		Run: func(a *app, args []string) error {
			a.emitValue(map[string]string{"version": version}, func() { fmt.Fprintln(a.out, "docklite", version) })
			return nil
		},
	})
	register(command{
		Path: []string{"help"}, Summary: "Show help for all commands, or one command",
		Usage: "help [command]", NoClient: true,
		Run: func(a *app, args []string) error {
			if len(args) == 0 {
				a.printUsage()
				return nil
			}
			cmd, _ := findCommand(args)
			if cmd != nil {
				a.printCommandHelp(cmd)
				return nil
			}
			if isGroup(args[0]) {
				a.printGroupHelp(args[0])
				return nil
			}
			return usageError("no such command %q", strings.Join(args, " "))
		},
	})
	register(command{
		Path: []string{"commands"}, Summary: "List every command as JSON (for scripts and AI assistants)",
		Usage: "commands", NoClient: true,
		Examples: []string{"docklite commands --json"},
		Run: func(a *app, args []string) error {
			out, _ := json.MarshalIndent(registry, "", "  ")
			fmt.Fprintln(a.out, string(out))
			return nil
		},
	})
	register(command{
		Path: []string{"docs"}, Summary: "Print the full usage guide (works without the repo; start here)",
		Usage: "docs", NoClient: true,
		Run: func(a *app, args []string) error {
			fmt.Fprintln(a.out, strings.TrimRight(cliGuide, "\n"))
			fmt.Fprintln(a.out)
			fmt.Fprintln(a.out, "## Command reference (generated from the program itself)")
			fmt.Fprintln(a.out)
			for _, c := range sortedCommands() {
				flags := ""
				if c.Destructive {
					flags += " [destructive: needs --yes]"
				}
				if c.AdminOnly {
					flags += " [admin]"
				}
				fmt.Fprintf(a.out, "- `docklite %s` — %s%s\n", c.Usage, c.Summary, flags)
				for _, ex := range c.Examples {
					fmt.Fprintf(a.out, "    e.g. `%s`\n", ex)
				}
			}
			return nil
		},
	})

	register(command{
		Path: []string{"login"}, Summary: "Log in with your DockLite account and save a token",
		Usage: "login [--username NAME] [--password-stdin] [--token-name NAME] [--expires-at RFC3339]",
		Examples: []string{
			"docklite login",
			"docklite --host https://xxl.docklite.net login",
			"echo \"$PASSWORD\" | docklite login --username me --password-stdin",
		},
		Run: runLogin,
	})
	register(command{
		Path: []string{"logout"}, Summary: "Forget the saved token for this server",
		Usage: "logout", NoClient: true, Run: runLogout,
	})
	register(command{
		Path: []string{"whoami"}, Summary: "Show which account and address the CLI is using, and why",
		Usage: "whoami",
		Run: func(a *app, args []string) error {
			data, err := a.get("/api/auth/me")
			if err != nil {
				return err
			}
			var resp struct {
				User map[string]any `json:"user"`
			}
			_ = json.Unmarshal(data, &resp)
			info := map[string]any{"host": a.client.BaseURL, "credentials": a.credSource, "user": resp.User}
			a.emitValue(info, func() {
				fmt.Fprintf(a.out, "server:       %s\n", a.client.BaseURL)
				fmt.Fprintf(a.out, "credentials:  %s\n", a.credSource)
				fmt.Fprintf(a.out, "user:         %v (%v)\n", resp.User["username"], resp.User["role"])
			})
			return nil
		},
	})
	register(command{
		Path: []string{"config"}, Summary: "Show or change saved servers (config show|get|set|reset)",
		Usage: "config [show|get <key>|set <key> <value>|reset]", NoClient: true,
		Examples: []string{"docklite config set servers.default.host https://xxl.docklite.net"},
		Run:      func(a *app, args []string) error { return runConfig(a, args) },
	})

	register(command{
		Path: []string{"status"}, Summary: "Agent status (note: `docklite status` as the launcher shows the services dashboard)",
		Usage: "status",
		Run:   func(a *app, args []string) error { return a.getAndEmit("/api/status") },
	})
	register(command{
		Path: []string{"info"}, Summary: "Summary: containers, sites, databases, users",
		Usage: "info",
		Run:   func(a *app, args []string) error { return a.getAndEmit("/api/summary") },
	})
	register(command{
		Path: []string{"tokens"}, Summary: "List API tokens", Usage: "tokens", AdminOnly: true,
		Run: func(a *app, args []string) error { return a.getAndEmit("/api/tokens") },
	})
	register(command{
		Path: []string{"token", "create"}, Summary: "Create an API token (shown once)",
		Usage: "token create <name>", AdminOnly: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite token create <name>")
			}
			data, err := a.post("/api/tokens", map[string]any{"name": args[0]})
			if err != nil {
				return err
			}
			a.emit(data)
			return nil
		},
	})
	register(command{
		Path: []string{"token", "revoke"}, Summary: "Revoke an API token by id",
		Usage: "token revoke <id>", AdminOnly: true, Destructive: true,
		Run: func(a *app, args []string) error {
			if len(args) != 1 {
				return usageError("usage: docklite token revoke <id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return usageError("token id must be a number (see: docklite tokens)")
			}
			if err := a.confirm(fmt.Sprintf("revoke token %d", id)); err != nil {
				return err
			}
			if _, err := a.post("/api/tokens/revoke", map[string]any{"id": id}); err != nil {
				return err
			}
			a.say("token %d revoked", id)
			return nil
		},
	})
}

func (a *app) getAndEmit(path string) error {
	data, err := a.get(path)
	if err != nil {
		return err
	}
	a.emit(data)
	return nil
}

func sortedCommands() []command {
	out := append([]command(nil), registry...)
	sort.Slice(out, func(i, j int) bool { return out[i].name() < out[j].name() })
	return out
}

func isGroup(name string) bool {
	for _, g := range groups() {
		if g == name {
			for _, c := range registry {
				if c.Path[0] == name && len(c.Path) > 1 {
					return true
				}
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Help

func (a *app) printUsage() {
	fmt.Fprintln(a.out, "docklite — manage a DockLite server from the command line")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Usage: docklite <command> [flags]")
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Commands:")
	shown := map[string]bool{}
	for _, c := range sortedCommands() {
		key := c.Path[0]
		if len(c.Path) > 1 {
			if shown[key] {
				continue
			}
			shown[key] = true
			fmt.Fprintf(a.out, "  %-18s %s\n", key+" ...", groupSummary(key))
			continue
		}
		fmt.Fprintf(a.out, "  %-18s %s\n", c.name(), c.Summary)
	}
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Flags for every command: --json  --yes  --quiet  --host URL  --token TOKEN  --server NAME  --timeout 30s")
	fmt.Fprintln(a.out, "New here (or an AI assistant)? Run: docklite docs")
}

func groupSummary(group string) string {
	var names []string
	for _, c := range sortedCommands() {
		if c.Path[0] == group && len(c.Path) > 1 {
			names = append(names, c.Path[1])
		}
	}
	return strings.Join(names, ", ")
}

func (a *app) printGroupHelp(group string) {
	fmt.Fprintf(a.out, "docklite %s <subcommand>\n\n", group)
	for _, c := range sortedCommands() {
		if c.Path[0] == group && len(c.Path) > 1 {
			fmt.Fprintf(a.out, "  %-34s %s\n", c.Usage, c.Summary)
		}
	}
}

func (a *app) printCommandHelp(c *command) {
	fmt.Fprintf(a.out, "docklite %s\n\n  %s\n", c.Usage, c.Summary)
	if c.AdminOnly {
		fmt.Fprintln(a.out, "\n  Needs a DockLite admin account.")
	}
	if c.Destructive {
		fmt.Fprintln(a.out, "\n  Destructive: asks for confirmation, or needs --yes when run from a script.")
	}
	if len(c.Examples) > 0 {
		fmt.Fprintln(a.out, "\nExamples:")
		for _, ex := range c.Examples {
			fmt.Fprintln(a.out, "  "+ex)
		}
	}
}

// ---------------------------------------------------------------------------
// login / logout / config

func runLogin(a *app, args []string) error {
	var username, tokenName, expiresAt string
	var passwordStdin bool
	if _, err := a.flags("login", args, func(fs *flag.FlagSet) {
		fs.StringVar(&username, "username", "", "")
		fs.StringVar(&tokenName, "token-name", "cli", "")
		fs.StringVar(&expiresAt, "expires-at", "", "")
		fs.BoolVar(&passwordStdin, "password-stdin", false, "")
	}); err != nil {
		return err
	}

	var err error
	if username == "" {
		if username, err = a.readLine("Username: "); err != nil {
			return fmt.Errorf("could not read the username: %w", err)
		}
	}
	var password string
	if passwordStdin {
		password, err = a.readLine("")
	} else {
		password, err = a.readSecret("Password: ")
	}
	if err != nil {
		return fmt.Errorf("could not read the password: %w", err)
	}

	payload := map[string]any{"username": username, "password": password, "issue_token": true, "token_name": tokenName}
	if strings.TrimSpace(expiresAt) != "" {
		payload["expires_at"] = strings.TrimSpace(expiresAt)
	}
	loginClient := *a.client
	loginClient.Token = ""
	a.client = &loginClient
	data, err := a.post("/api/auth/login", payload)
	if err != nil {
		return err
	}

	var resp struct {
		Token struct {
			Secret string `json:"secret"`
		} `json:"token"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || strings.TrimSpace(resp.Token.Secret) == "" {
		return fmt.Errorf("login worked but the server sent no token")
	}

	name := a.opts.Server
	if name == "" {
		name = a.cfg.CurrentServer
	}
	if name == "" {
		name = "default"
	}
	if a.cfg.Servers == nil {
		a.cfg.Servers = map[string]cli.ServerConfig{}
	}
	server := a.cfg.Servers[name]
	// An address given on this command line is the one to remember.
	if a.opts.Host != "" || server.Host == "" {
		server.Host = a.client.BaseURL
	}
	server.Token = resp.Token.Secret
	a.cfg.Servers[name] = server
	if a.cfg.CurrentServer == "" {
		a.cfg.CurrentServer = name
	}
	if err := cli.SaveConfig(a.cfg, a.cfgPath); err != nil {
		return fmt.Errorf("logged in, but could not save the token: %w", err)
	}
	a.emitValue(map[string]any{"server": name, "host": server.Host, "saved": true}, func() {
		a.say("logged in; token saved for %s (%s)", name, server.Host)
	})
	return nil
}

func runLogout(a *app, args []string) error {
	name := a.opts.Server
	if name == "" {
		name = a.cfg.CurrentServer
	}
	if name == "" {
		name = "default"
	}
	server, ok := a.cfg.Servers[name]
	if !ok {
		return &exitError{code: exitNotFound, msg: "no saved server called " + name}
	}
	server.Token = ""
	a.cfg.Servers[name] = server
	if err := cli.SaveConfig(a.cfg, a.cfgPath); err != nil {
		return err
	}
	a.say("token cleared for %s", name)
	return nil
}

func runConfig(a *app, args []string) error {
	if len(args) == 0 || args[0] == "show" {
		// Never print tokens: this output ends up in terminals and transcripts.
		masked := *a.cfg
		masked.Servers = map[string]cli.ServerConfig{}
		for name, s := range a.cfg.Servers {
			if s.Token != "" {
				s.Token = "(saved)"
			}
			masked.Servers[name] = s
		}
		out, _ := json.MarshalIndent(masked, "", "  ")
		fmt.Fprintln(a.out, string(out))
		return nil
	}
	switch args[0] {
	case "set":
		if len(args) != 3 {
			return usageError("usage: docklite config set <key> <value>")
		}
		if err := setConfigValue(a.cfg, args[1], args[2]); err != nil {
			return usageError("%v", err)
		}
		return cli.SaveConfig(a.cfg, a.cfgPath)
	case "get":
		if len(args) != 2 {
			return usageError("usage: docklite config get <key>")
		}
		value, err := getConfigValue(a.cfg, args[1])
		if err != nil {
			return usageError("%v", err)
		}
		fmt.Fprintln(a.out, value)
		return nil
	case "reset":
		fresh := &cli.Config{CurrentServer: "default", Servers: map[string]cli.ServerConfig{"default": {Host: cli.DefaultHost()}}}
		return cli.SaveConfig(fresh, a.cfgPath)
	}
	return usageError("unknown config subcommand %q (show, get, set, reset)", args[0])
}

func setConfigValue(cfg *cli.Config, key, value string) error {
	if key == "current_server" {
		cfg.CurrentServer = value
		return nil
	}
	parts := strings.Split(key, ".")
	if len(parts) != 3 || parts[0] != "servers" {
		return fmt.Errorf("unknown key %q (try current_server or servers.<name>.host)", key)
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]cli.ServerConfig{}
	}
	server := cfg.Servers[parts[1]]
	switch parts[2] {
	case "host":
		server.Host = value
	case "token":
		server.Token = value
	default:
		return fmt.Errorf("unknown server field %q (host or token)", parts[2])
	}
	cfg.Servers[parts[1]] = server
	return nil
}

func getConfigValue(cfg *cli.Config, key string) (string, error) {
	if key == "current_server" {
		return cfg.CurrentServer, nil
	}
	parts := strings.Split(key, ".")
	if len(parts) != 3 || parts[0] != "servers" {
		return "", fmt.Errorf("unknown key %q", key)
	}
	server, ok := cfg.Servers[parts[1]]
	if !ok {
		return "", fmt.Errorf("no saved server called %q", parts[1])
	}
	switch parts[2] {
	case "host":
		return server.Host, nil
	case "token":
		if server.Token == "" {
			return "", nil
		}
		return "(saved — not shown)", nil
	}
	return "", fmt.Errorf("unknown server field %q", parts[2])
}
