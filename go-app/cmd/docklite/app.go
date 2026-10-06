package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"docklite-agent/internal/cli"

	"golang.org/x/crypto/ssh/terminal"
)

// Exit codes. Scripts (and AI assistants) can rely on these:
const (
	exitOK          = 0
	exitFailure     = 1 // anything that went wrong and isn't covered below
	exitUsage       = 2 // wrong arguments or flags
	exitAuth        = 3 // not logged in, or not allowed
	exitNotFound    = 4 // the thing you named doesn't exist
	exitConflict    = 5 // the server refused: already exists, or not allowed in this state
	exitUnreachable = 6 // couldn't reach the DockLite agent at all
	exitNeedsYes    = 7 // a destructive command was run without --yes and no one could be asked
)

// exitError carries a specific exit code out of a command.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func usageError(format string, args ...any) error {
	return &exitError{code: exitUsage, msg: fmt.Sprintf(format, args...)}
}

type globalOptions struct {
	Server  string
	Host    string
	Token   string
	JSON    bool
	Quiet   bool
	Verbose bool
	NoColor bool
	Timeout time.Duration
	Yes     bool
}

type app struct {
	opts       globalOptions
	in         io.Reader
	out        io.Writer
	errOut     io.Writer
	cfg        *cli.Config
	cfgPath    string
	client     *cli.Client
	credSource string
	getenv     func(string) string
}

// extractGlobals pulls the flags every command shares out of the argument
// list wherever they appear, so `docklite containers list --json` works as
// naturally as `docklite --json containers list`.
func extractGlobals(args []string) (globalOptions, []string, error) {
	opts := globalOptions{Timeout: 30 * time.Second}
	var rest []string
	valueFlag := func(name string, i *int) (string, bool, error) {
		arg := args[*i]
		if arg == "--"+name {
			if *i+1 >= len(args) {
				return "", true, usageError("--%s needs a value", name)
			}
			*i++
			return args[*i], true, nil
		}
		if strings.HasPrefix(arg, "--"+name+"=") {
			return strings.TrimPrefix(arg, "--"+name+"="), true, nil
		}
		return "", false, nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
			continue
		case "--quiet", "-q":
			opts.Quiet = true
			continue
		case "--verbose":
			opts.Verbose = true
			continue
		case "--no-color":
			opts.NoColor = true
			continue
		case "--yes", "-y":
			opts.Yes = true
			continue
		}
		matched := false
		for _, name := range []string{"server", "host", "token", "timeout"} {
			value, ok, err := valueFlag(name, &i)
			if err != nil {
				return opts, nil, err
			}
			if !ok {
				continue
			}
			matched = true
			switch name {
			case "server":
				opts.Server = value
			case "host":
				opts.Host = value
			case "token":
				opts.Token = value
			case "timeout":
				d, err := time.ParseDuration(value)
				if err != nil {
					return opts, nil, usageError("--timeout must look like 30s or 2m, not %q", value)
				}
				opts.Timeout = d
			}
			break
		}
		if !matched {
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}

// flags parses a command's own flags, allowing them before, between or after
// positional arguments. It returns the positional arguments.
func (a *app) flags(name string, args []string, define func(fs *flag.FlagSet)) ([]string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if define != nil {
		define(fs)
	}
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usageError("%s: %v (see: docklite help %s)", name, err, name)
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

// ---------------------------------------------------------------------------
// Talking to the agent

func (a *app) call(method, path string, payload any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.opts.Timeout)
	defer cancel()
	data, err := a.client.Do(ctx, method, path, payload)
	if err != nil {
		return nil, a.classify(err)
	}
	return data, nil
}

func (a *app) get(path string) ([]byte, error)         { return a.call("GET", path, nil) }
func (a *app) post(path string, p any) ([]byte, error) { return a.call("POST", path, p) }
func (a *app) put(path string, p any) ([]byte, error)  { return a.call("PUT", path, p) }
func (a *app) del(path string) ([]byte, error)         { return a.call("DELETE", path, nil) }

// classify turns client errors into exit codes with a message that says what
// to do next.
func (a *app) classify(err error) error {
	var apiErr *cli.APIError
	var unreachable *cli.UnreachableError
	switch {
	case errors.As(err, &unreachable):
		return &exitError{code: exitUnreachable, msg: unreachable.Error() + "\n  Is DockLite running? Try: docklite status   (or set the address: docklite config set servers.default.host https://your-server)"}
	case errors.As(err, &apiErr):
		switch apiErr.Status {
		case 401:
			return &exitError{code: exitAuth, msg: "not logged in (" + apiErr.Error() + ")\n  Run: docklite login"}
		case 403:
			return &exitError{code: exitAuth, msg: "not allowed: " + apiErr.Error() + "\n  This needs a DockLite admin account."}
		case 404:
			return &exitError{code: exitNotFound, msg: apiErr.Error()}
		case 409:
			return &exitError{code: exitConflict, msg: apiErr.Error()}
		case 400:
			return &exitError{code: exitConflict, msg: apiErr.Error()}
		}
		return &exitError{code: exitFailure, msg: fmt.Sprintf("%s (HTTP %d)", apiErr.Error(), apiErr.Status)}
	}
	return err
}

// ---------------------------------------------------------------------------
// Output

func (a *app) say(format string, args ...any) {
	if a.opts.Quiet {
		return
	}
	fmt.Fprintf(a.out, format+"\n", args...)
}

// emit prints an API response: raw with --json, otherwise pretty-printed.
func (a *app) emit(data []byte) {
	if a.opts.JSON {
		fmt.Fprintln(a.out, strings.TrimSpace(string(data)))
		return
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Fprintln(a.out, strings.TrimSpace(string(data)))
		return
	}
	pretty, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(a.out, string(pretty))
}

// emitValue prints a Go value as JSON (--json) or via the human renderer.
func (a *app) emitValue(v any, human func()) {
	if a.opts.JSON {
		out, _ := json.Marshal(v)
		fmt.Fprintln(a.out, string(out))
		return
	}
	human()
}

func (a *app) table(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

// ---------------------------------------------------------------------------
// Confirmation and input

func (a *app) stdinIsTerminal() bool {
	f, ok := a.in.(*os.File)
	return ok && terminal.IsTerminal(int(f.Fd()))
}

// confirm guards destructive commands. With --yes it passes; at a terminal
// it asks; anywhere else (a script, an AI assistant) it refuses, so nothing
// destructive happens by accident.
func (a *app) confirm(what string) error {
	if a.opts.Yes {
		return nil
	}
	if !a.stdinIsTerminal() {
		return &exitError{code: exitNeedsYes, msg: fmt.Sprintf("refusing to %s without --yes (nothing was changed)", what)}
	}
	fmt.Fprintf(a.errOut, "%s? [y/N] ", strings.ToUpper(what[:1])+what[1:])
	line, _ := bufio.NewReader(a.in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return &exitError{code: exitFailure, msg: "cancelled (nothing was changed)"}
}

func (a *app) readLine(prompt string) (string, error) {
	fmt.Fprint(a.errOut, prompt)
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a *app) readSecret(prompt string) (string, error) {
	if f, ok := a.in.(*os.File); ok && terminal.IsTerminal(int(f.Fd())) {
		fmt.Fprint(a.errOut, prompt)
		value, err := terminal.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.errOut)
		return strings.TrimSpace(string(value)), err
	}
	return a.readLine(prompt)
}

// ---------------------------------------------------------------------------
// The command registry: one list drives help, `docklite commands --json`,
// `docklite docs` and dispatch, so they can never disagree.

type command struct {
	Path        []string `json:"path"`
	Summary     string   `json:"summary"`
	Usage       string   `json:"usage"`
	Examples    []string `json:"examples,omitempty"`
	Destructive bool     `json:"destructive"`
	AdminOnly   bool     `json:"adminOnly"`
	// NoClient commands run without credentials (help, docs, config...).
	NoClient bool                              `json:"-"`
	Run      func(a *app, args []string) error `json:"-"`
}

var registry []command

func register(c command) { registry = append(registry, c) }

func (c command) name() string { return strings.Join(c.Path, " ") }

// findCommand matches the longest registered path at the start of args.
func findCommand(args []string) (*command, []string) {
	var best *command
	for i := range registry {
		c := &registry[i]
		if len(c.Path) > len(args) {
			continue
		}
		match := true
		for j, part := range c.Path {
			if args[j] != part {
				match = false
				break
			}
		}
		if match && (best == nil || len(c.Path) > len(best.Path)) {
			best = c
		}
	}
	if best == nil {
		return nil, args
	}
	return best, args[len(best.Path):]
}

// groups lists the first words of all registered commands, in a stable order.
func groups() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range registry {
		if !seen[c.Path[0]] {
			seen[c.Path[0]] = true
			out = append(out, c.Path[0])
		}
	}
	sort.Strings(out)
	return out
}
