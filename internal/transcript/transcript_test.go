package transcript

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func bashLine(session, cwd, timestamp, id, command string) string {
	return `{"type":"assistant","sessionId":"` + session + `","cwd":"` + cwd + `","timestamp":"` + timestamp + `",` +
		`"message":{"role":"assistant","content":[{"type":"text","text":"checking"},` +
		`{"type":"tool_use","id":"` + id + `","name":"Bash","input":{"command":"` + command + `","description":"d"}}]}}`
}

func TestParseLine(t *testing.T) {
	calls := ParseLine([]byte(bashLine("s1", "/home/me/shop", "2026-01-02T03:04:05.678Z", "t1", "git status")))
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
	want := Call{
		ID: "t1", Session: "s1", Directory: "/home/me/shop",
		Time: time.Date(2026, 1, 2, 3, 4, 5, 678e6, time.UTC), Command: "git status",
	}
	if calls[0] != want {
		t.Fatalf("got %+v, want %+v", calls[0], want)
	}
}

func TestParseLineIgnoresOtherLines(t *testing.T) {
	lines := []string{
		`{"type":"user","message":{"content":"run \"Bash\" please"}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"Bash"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"Bash"}}]}}`,
		`{"type":"assistant","message":{"content":"plain text about Bash"}}`,
		`{"type":"api-request-blob","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"assistant", "message": {"content": [{"type":"tool_use","name":"Bash"`,
	}
	for _, line := range lines {
		if calls := ParseLine([]byte(line)); calls != nil {
			t.Errorf("ParseLine(%s) = %+v, want none", line, calls)
		}
	}
}

func write(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commandsOf(calls []Call) []string {
	var out []string
	for _, call := range calls {
		out = append(out, call.Command)
	}
	return out
}

func sampleProjects(t *testing.T) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "-home-me", "s1.jsonl"),
		`{"type":"user","message":{"content":"hello"}}`,
		bashLine("s1", "/home/me/shop", "2026-03-01T10:00:00Z", "a", "git status"),
		bashLine("s1", "/home/me/shop", "2026-03-01T10:01:00Z", "b", "git log"),
	)
	write(t, filepath.Join(root, "-home-me", "s1", "subagents", "agent-1.jsonl"),
		bashLine("s1", "/home/me/shop", "2026-03-01T10:02:00Z", "c", "make test"),
	)
	write(t, filepath.Join(root, "-home-me", "s2.jsonl"),
		bashLine("s2", "/home/me/shop", "2026-03-01T10:00:00Z", "a", "git status"),
		bashLine("s2", "/home/me/blog/theme", "2026-03-05T09:01:00Z", "d", "go test"),
	)
	write(t, filepath.Join(root, "-home-me-blog", "s3.jsonl"),
		bashLine("s3", "/home/me/blog", "2026-01-01T00:00:00Z", "e", "hugo serve"),
	)
	write(t, filepath.Join(root, "-home-me-blog", "notes.txt"),
		bashLine("s3", "/home/me/blog", "2026-01-01T00:00:00Z", "f", "ls"))
	return root
}

func TestRead(t *testing.T) {
	calls, err := Read(sampleProjects(t), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	// Name order, so the subagent directory s1 comes before s1.jsonl; the call that s2
	// repeats from s1 is read once.
	want := []string{"make test", "git status", "git log", "go test", "hugo serve"}
	if got := commandsOf(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if !strings.HasSuffix(calls[0].Transcript, "agent-1.jsonl") || calls[0].Session != "s1" {
		t.Fatalf("subagent call has transcript %q and session %q", calls[0].Transcript, calls[0].Session)
	}
}

func TestReadFiltersByProjectAndTime(t *testing.T) {
	root := sampleProjects(t)

	calls, err := Read(root, Filter{Project: "blog"})
	if err != nil {
		t.Fatal(err)
	}
	// The project is where a command ran, even when the session started elsewhere.
	if got := commandsOf(calls); !reflect.DeepEqual(got, []string{"go test", "hugo serve"}) {
		t.Fatalf("project filter: got %q", got)
	}

	calls, err = Read(root, Filter{Since: time.Date(2026, 3, 1, 10, 0, 30, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"make test", "git log", "go test"}
	if got := commandsOf(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("time filter: got %q, want %q", got, want)
	}
}

func TestReadMissingDirectory(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "missing"), Filter{}); err == nil {
		t.Fatal("no error for a missing directory")
	}
}
