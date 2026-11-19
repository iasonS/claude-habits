// Package transcript reads the shell commands the assistant ran through the Bash tool out of
// Claude Code session transcripts: one JSON object per line, in .jsonl files under
// ~/.claude/projects/<project>/.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A single line can hold a large tool result; this only caps how far the buffer may grow.
const maxLine = 1 << 30

// Lines without this text cannot hold a Bash call, so they are not parsed.
var bashMarker = []byte(`"Bash"`)

// Call is one Bash tool call.
type Call struct {
	ID         string
	Session    string
	Transcript string // the file the call was read from; one file is one agent's conversation
	Directory  string // the working directory the command ran in
	Time       time.Time
	Command    string
}

// Filter limits which calls Read returns.
type Filter struct {
	Project string    // part of the working directory; empty matches every directory
	Since   time.Time // calls before this time are left out; zero keeps all
}

type entry struct {
	Type      string    `json:"type"`
	SessionID string    `json:"sessionId"`
	Cwd       string    `json:"cwd"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input struct {
		Command string `json:"command"`
	} `json:"input"`
}

// ParseLine returns the Bash calls in one transcript line. Lines that are not assistant
// messages, or that are not valid JSON, have none.
func ParseLine(line []byte) []Call {
	var e entry
	if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
		return nil
	}
	var blocks []block
	if json.Unmarshal(e.Message.Content, &blocks) != nil {
		return nil
	}
	var calls []Call
	for _, b := range blocks {
		if b.Type != "tool_use" || b.Name != "Bash" || b.Input.Command == "" {
			continue
		}
		calls = append(calls, Call{
			ID: b.ID, Session: e.SessionID, Directory: e.Cwd, Time: e.Timestamp, Command: b.Input.Command,
		})
	}
	return calls
}

type reader struct {
	filter Filter
	seen   map[string]bool // call IDs already read; a resumed session repeats earlier calls
	calls  []Call
}

// Read returns the Bash calls of every transcript under root, file by file in name order and
// each file in the order it was written.
func Read(root string, filter Filter) ([]Call, error) {
	r := &reader{filter: filter, seen: map[string]bool{}}
	err := filepath.WalkDir(root, r.visit)
	return r.calls, err
}

func (r *reader) visit(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
		return nil
	}
	info, err := entry.Info()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// A file last written before the cutoff cannot hold a newer call.
	if info.ModTime().Before(r.filter.Since) {
		return nil
	}
	return r.readFile(path)
}

func (r *reader) readFile(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), maxLine)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, bashMarker) {
			continue
		}
		for _, call := range ParseLine(line) {
			r.add(call, path)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (r *reader) add(call Call, path string) {
	if call.Time.Before(r.filter.Since) || !strings.Contains(call.Directory, r.filter.Project) {
		return
	}
	if call.ID != "" && r.seen[call.ID] {
		return
	}
	r.seen[call.ID] = true
	call.Transcript = path
	r.calls = append(r.calls, call)
}
