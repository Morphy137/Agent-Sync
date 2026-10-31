// Package cli implements the agentsync-cli command line: the same operations as the GUI, through
// internal/ops, with human-readable output (en/es) or a stable JSON envelope for AI agents (--json).
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/mcpserver"
	"agentsync/internal/ops"
)

// Version is the CLI version; overridden at build time with -ldflags "-X agentsync/internal/cli.Version=...".
var Version = "dev"

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Global flags.
const (
	flagJSON = "--json"
	flagLang = "--lang"
	flagHelp = "--help"
)

// LangEnvVar forces the CLI language (en or es).
const LangEnvVar = "AGENTSYNC_LANG"

// APIFactory creates the ops.API lazily so help and usage errors never touch the config.
type APIFactory func() *ops.API

// Envelope is the JSON shape printed with --json.
type Envelope struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

// ErrorBody describes a failure in JSON output.
type ErrorBody struct {
	Code    string            `json:"code"`
	Params  map[string]string `json:"params,omitempty"`
	Message string            `json:"message"`
}

// usageError marks invalid command lines (exit code 2).
type usageError struct {
	msg string
}

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// env is everything a command needs.
type env struct {
	ctx     context.Context
	factory APIFactory
	api     *ops.API
	stdout  io.Writer
	lang    string
	langSet bool
}

// API returns the shared ops.API, creating it on first use (and adopting the configured language).
func (e *env) API() *ops.API {
	if e.api == nil {
		e.api = e.factory()
		if !e.langSet {
			e.lang = resolveLang("", e.api.Language())
		}
	}
	return e.api
}

// Run executes the command line and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, factory APIFactory) int {
	jsonOut, langFlag, help, rest := splitGlobalFlags(args)
	e := &env{ctx: ctx, factory: factory, stdout: stdout, lang: resolveLang(langFlag, ""), langSet: langFlag != "" || os.Getenv(LangEnvVar) != ""}

	if help || len(rest) == 0 || rest[0] == "help" {
		topic := rest
		if len(topic) > 0 && topic[0] == "help" {
			topic = topic[1:]
		}
		printHelp(stdout, e.lang, topic)
		return ExitOK
	}

	cmd, cmdArgs := findCommand(rest)
	if cmd == nil {
		return fail(e, stdout, stderr, jsonOut, usagef("%s: %s", tr(e.lang, "err.unknown_command"), strings.Join(rest, " ")))
	}
	if cmd.raw != nil {
		if err := cmd.raw(e, cmdArgs); err != nil {
			return fail(e, stdout, stderr, jsonOut, err)
		}
		return ExitOK
	}
	result, err := cmd.run(e, cmdArgs)
	if err != nil {
		return fail(e, stdout, stderr, jsonOut, err)
	}
	if jsonOut {
		writeJSON(stdout, Envelope{OK: true, Data: result})
		return ExitOK
	}
	render(stdout, e.lang, result)
	return ExitOK
}

// Main is the entry point used by cmd/agentsync-cli.
func Main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return Run(ctx, os.Args[1:], os.Stdout, os.Stderr, ops.NewDefault)
}

// splitGlobalFlags extracts --json, --lang and --help from anywhere in the command line.
func splitGlobalFlags(args []string) (jsonOut bool, lang string, help bool, rest []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == flagJSON:
			jsonOut = true
		case arg == flagHelp || arg == "-h":
			help = true
		case arg == flagLang && i+1 < len(args):
			lang = args[i+1]
			i++
		case strings.HasPrefix(arg, flagLang+"="):
			lang = strings.TrimPrefix(arg, flagLang+"=")
		default:
			rest = append(rest, arg)
		}
	}
	return jsonOut, lang, help, rest
}

func fail(e *env, stdout, stderr io.Writer, jsonOut bool, err error) int {
	code := ExitError
	var ue *usageError
	if errors.As(err, &ue) {
		code = ExitUsage
	}
	body := errorBody(err, code == ExitUsage)
	if jsonOut {
		writeJSON(stdout, Envelope{OK: false, Error: body})
		return code
	}
	fmt.Fprintf(stderr, "%s: %s\n", tr(e.lang, "label.error"), body.Message)
	if code == ExitUsage {
		fmt.Fprintln(stderr, tr(e.lang, "hint.help"))
	}
	return code
}

func errorBody(err error, usage bool) *ErrorBody {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return &ErrorBody{Code: ae.Code, Params: ae.Params, Message: ae.Error()}
	}
	if usage {
		return &ErrorBody{Code: "usage", Message: err.Error()}
	}
	return &ErrorBody{Code: apperr.Unknown, Message: err.Error()}
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// ---- Flag helpers ----

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseArgs parses flags placed before, between or after positional arguments.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usagef("%s: %v", fs.Name(), err)
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

// expectArgs parses flags and checks the positional argument count.
func expectArgs(fs *flag.FlagSet, args []string, names ...string) ([]string, error) {
	pos, err := parseArgs(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != len(names) {
		return nil, usagef("%s: expected %s", fs.Name(), strings.Join(names, " "))
	}
	return pos, nil
}

func requireYes(name string, yes bool) error {
	if yes {
		return nil
	}
	return apperr.New(apperr.ConfirmationRequired).WithParam("hint", "re-run "+name+" with --yes")
}

func runMCP(e *env, args []string) error {
	if _, err := expectArgs(newFlags("mcp"), args); err != nil {
		return err
	}
	return mcpserver.New(e.API(), Version).Run(e.ctx)
}
