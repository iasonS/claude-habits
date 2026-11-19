// claude-habits ranks the shell commands Claude Code ran through its Bash tool, so the
// repeated chores can become a script, an alias or a permission rule.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iasonS/claude-habits/internal/habits"
	"github.com/iasonS/claude-habits/internal/transcript"
)

const usageText = `Usage: claude-habits [report|sequences|allowlist] [flags]

  report     the most frequent command shapes (the default)
  sequences  shapes that often run one right after the other
  allowlist  frequent read-only shapes as Claude Code permission rules

Flags:
`

var commands = map[string]bool{"report": true, "sequences": true, "allowlist": true}

type options struct {
	command string
	dir     string
	project string
	since   time.Duration
	top     int
	json    bool
	all     bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "claude-habits:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}

	filter := transcript.Filter{Project: opts.project}
	if opts.since > 0 {
		filter.Since = time.Now().Add(-opts.since)
	}
	calls, err := transcript.Read(opts.dir, filter)
	if err != nil {
		return err
	}
	shaped := shapeCalls(calls, opts.all)
	if !opts.json {
		fmt.Fprintf(out, "%d Bash calls in %d sessions\n\n", len(calls), countSessions(calls))
	}

	switch opts.command {
	case "sequences":
		return printPairs(out, first(habits.Sequences(shaped), opts.top), opts.json)
	case "allowlist":
		return printAllowlist(out, first(habits.ReadOnly(habits.Top(shaped)), opts.top), opts.json)
	default:
		return printCounts(out, first(habits.Top(shaped), opts.top), opts.json)
	}
}

// parseArgs accepts the flags before or after the command name.
func parseArgs(args []string) (options, error) {
	opts := options{command: "report"}
	home, _ := os.UserHomeDir()
	flags := flag.NewFlagSet("claude-habits", flag.ExitOnError)
	flags.StringVar(&opts.dir, "dir", filepath.Join(home, ".claude", "projects"), "directory with the Claude Code transcripts")
	flags.StringVar(&opts.project, "project", "", "only commands run in a directory whose path contains this text")
	since := flags.String("since", "", "only calls from this recent period, such as 30d or 12h")
	flags.IntVar(&opts.top, "top", 25, "number of rows to print, 0 for all")
	flags.BoolVar(&opts.json, "json", false, "print JSON")
	flags.BoolVar(&opts.all, "all", false, "include shell noise such as cd, echo, grep and loop keywords")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usageText)
		flags.PrintDefaults()
	}

	flags.Parse(args)
	if flags.NArg() > 0 {
		opts.command = flags.Arg(0)
		flags.Parse(flags.Args()[1:])
	}
	if flags.NArg() > 0 {
		return opts, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if !commands[opts.command] {
		return opts, fmt.Errorf("unknown command %q; want report, sequences or allowlist", opts.command)
	}

	var err error
	opts.since, err = parseSince(*since)
	return opts, err
}

// parseSince reads a period such as 30d, 12h or 90m; empty means no limit.
func parseSince(text string) (time.Duration, error) {
	if text == "" {
		return 0, nil
	}
	if days, found := strings.CutSuffix(text, "d"); found {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--since %q: want a number of days such as 30d", text)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	period, err := time.ParseDuration(text)
	if err != nil || period <= 0 {
		return 0, fmt.Errorf("--since %q: want a period such as 30d or 12h", text)
	}
	return period, nil
}

func shapeCalls(calls []transcript.Call, withNoise bool) []habits.Call {
	shaped := make([]habits.Call, 0, len(calls))
	for _, c := range calls {
		shaped = append(shaped, habits.Call{
			Session:    c.Session,
			Transcript: c.Transcript,
			Shapes:     habits.Shapes(c.Command, withNoise),
		})
	}
	return shaped
}

func countSessions(calls []transcript.Call) int {
	sessions := map[string]bool{}
	for _, c := range calls {
		sessions[c.Session] = true
	}
	return len(sessions)
}

// first returns the first n items, or all of them when n is 0.
func first[T any](items []T, n int) []T {
	if n <= 0 || n >= len(items) {
		return items
	}
	return items[:n]
}

func printCounts(out io.Writer, counts []habits.Count, asJSON bool) error {
	if asJSON {
		return writeJSON(out, counts)
	}
	fmt.Fprintf(out, "%7s %8s  %s\n", "COUNT", "SESSIONS", "SHAPE")
	for _, c := range counts {
		fmt.Fprintf(out, "%7d %8d  %s\n", c.Count, c.Sessions, c.Shape)
	}
	return nil
}

func printPairs(out io.Writer, pairs []habits.Pair, asJSON bool) error {
	if asJSON {
		return writeJSON(out, pairs)
	}
	fmt.Fprintf(out, "%7s %8s  %s\n", "COUNT", "SESSIONS", "FIRST -> THEN")
	for _, p := range pairs {
		fmt.Fprintf(out, "%7d %8d  %s -> %s\n", p.Count, p.Sessions, p.First, p.Then)
	}
	return nil
}

// printAllowlist prints a table, or with JSON the part of settings.json that allows the shapes.
func printAllowlist(out io.Writer, counts []habits.Count, asJSON bool) error {
	if asJSON {
		rules := make([]string, 0, len(counts))
		for _, c := range counts {
			rules = append(rules, habits.Rule(c.Shape))
		}
		return writeJSON(out, map[string]any{"permissions": map[string]any{"allow": rules}})
	}
	fmt.Fprintf(out, "%7s %8s  %s\n", "COUNT", "SESSIONS", "RULE")
	for _, c := range counts {
		fmt.Fprintf(out, "%7d %8d  %s\n", c.Count, c.Sessions, habits.Rule(c.Shape))
	}
	return nil
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
