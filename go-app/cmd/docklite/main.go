package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"docklite-agent/internal/cli"
)

// version is set at build time (-ldflags "-X main.version=...") from VERSION.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process: it returns the exit code, which is what
// lets the tests drive every command.
func run(args []string, in io.Reader, out, errOut io.Writer) int {
	a := &app{in: in, out: out, errOut: errOut, getenv: os.Getenv}

	opts, rest, err := extractGlobals(args)
	a.opts = opts
	if err != nil {
		return a.finish(err)
	}
	if len(rest) == 0 {
		a.printUsage()
		return exitOK
	}

	cmd, remaining := findCommand(rest)
	if cmd == nil {
		if isGroup(rest[0]) {
			a.printGroupHelp(rest[0])
			return exitOK
		}
		return a.finish(usageError("unknown command %q — see: docklite help", strings.Join(rest, " ")))
	}
	for _, arg := range remaining {
		if arg == "-h" || arg == "--help" {
			a.printCommandHelp(cmd)
			return exitOK
		}
	}

	a.cfg, a.cfgPath, err = cli.LoadConfig()
	if err != nil {
		return a.finish(fmt.Errorf("could not read the config file: %w", err))
	}
	a.resolveClient()

	return a.finish(cmd.Run(a, remaining))
}

func (a *app) finish(err error) int {
	if err == nil {
		return exitOK
	}
	code := exitFailure
	if ee, ok := err.(*exitError); ok {
		code = ee.code
	}
	if a.opts.JSON {
		fmt.Fprintf(a.errOut, "{\"error\":%q,\"code\":%d}\n", err.Error(), code)
	} else {
		fmt.Fprintln(a.errOut, "error: "+err.Error())
	}
	return code
}

// ---------------------------------------------------------------------------
// Credentials

const localConfPath = "/opt/docklite/.docklite.conf"

type localAdmin struct {
	Host  string
	Token string
}

// readLocalAdmin reads the server's own DockLite config. Only users who can
// read that file (the docklite user, or members of the docklite group — see
// `docklite access`) get this "admin shell access".
func readLocalAdmin(getenv func(string) string) *localAdmin {
	path := getenv("DOCKLITE_CONF")
	if path == "" {
		path = localConfPath
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	if values["DOCKLITE_TOKEN"] == "" {
		return nil
	}
	port := values["AGENT_PORT"]
	if port == "" {
		port = "3000"
	}
	return &localAdmin{Host: "http://127.0.0.1:" + port, Token: values["DOCKLITE_TOKEN"]}
}

// resolveClient picks the address and credential, in this order:
//  1. --host / --token flags, DOCKLITE_HOST / DOCKLITE_TOKEN environment
//  2. the saved profile (docklite login)
//  3. local admin access, if this user can read the server's config file
func (a *app) resolveClient() {
	host := strings.TrimSpace(a.opts.Host)
	token := strings.TrimSpace(a.opts.Token)
	source := ""
	if host == "" {
		host = strings.TrimSpace(a.getenv("DOCKLITE_HOST"))
	}
	if token == "" {
		token = strings.TrimSpace(a.getenv("DOCKLITE_TOKEN"))
	}
	if token != "" {
		source = "token from --token or DOCKLITE_TOKEN"
	}
	explicitHost := host != ""

	profile := a.opts.Server
	if profile == "" {
		profile = a.cfg.CurrentServer
	}
	profileHostIsDefault := true
	if server, ok := a.cfg.Servers[profile]; ok {
		if host == "" && server.Host != "" {
			host = server.Host
			profileHostIsDefault = server.Host == cli.DefaultHost()
		}
		if token == "" && server.Token != "" {
			token = server.Token
			source = fmt.Sprintf("saved login for server %q", profile)
		}
	}

	// A user who can read the server's own config gets admin access with no
	// login — but only when pointed at this machine, never a remote server.
	if token == "" && !explicitHost && profileHostIsDefault {
		if local := readLocalAdmin(a.getenv); local != nil {
			host, token = local.Host, local.Token
			source = "local admin access (you can read the server's DockLite config)"
		}
	}
	if host == "" {
		host = cli.DefaultHost()
	}
	if source == "" {
		source = "none — run: docklite login"
	}
	a.client = &cli.Client{BaseURL: host, Token: token, Timeout: a.opts.Timeout}
	a.credSource = source
}
