package habits

import (
	"reflect"
	"testing"
)

func TestShapes(t *testing.T) {
	command := "cd repo && git status | head -5\nkubectl -n web get pods -o name | grep api"
	if got, want := Shapes(command, false), []string{"git status", "kubectl get pods"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("without noise: got %q, want %q", got, want)
	}
	want := []string{"cd", "git status", "head", "kubectl get pods", "grep"}
	if got := Shapes(command, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("with noise: got %q, want %q", got, want)
	}
}

func TestShapesSkipsHeredocBodies(t *testing.T) {
	command := "python3 - <<'EOF'\nimport json\nfor row in rows: print(row)\nEOF"
	if got := Shapes(command, true); !reflect.DeepEqual(got, []string{"python3"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTop(t *testing.T) {
	calls := []Call{
		{Session: "s1", Shapes: []string{"git status", "git log"}},
		{Session: "s1", Shapes: []string{"git status"}},
		{Session: "s2", Shapes: []string{"git log"}},
		{Session: "s2", Shapes: []string{"make test"}},
	}
	want := []Count{
		{Shape: "git log", Count: 2, Sessions: 2},
		{Shape: "git status", Count: 2, Sessions: 1},
		{Shape: "make test", Count: 1, Sessions: 1},
	}
	if got := Top(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSequences(t *testing.T) {
	start, results := "aws logs start-query", "aws logs get-query-results"
	calls := []Call{
		{Session: "s1", Transcript: "a", Shapes: []string{start}},
		{Session: "s1", Transcript: "a", Shapes: []string{results}},
		{Session: "s1", Transcript: "a", Shapes: []string{results}},
		{Session: "s2", Transcript: "b", Shapes: []string{"git status"}},
		{Session: "s2", Transcript: "b", Shapes: []string{start, results}},
		// Another transcript of the same session does not continue the sequence of "b".
		{Session: "s2", Transcript: "c", Shapes: []string{"make test"}},
	}
	want := []Pair{
		{First: start, Then: results, Count: 2, Sessions: 2},
		{First: "git status", Then: start, Count: 1, Sessions: 1},
	}
	if got := Sequences(calls); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReadOnlyKeepsOrderAndDropsWrites(t *testing.T) {
	counts := []Count{
		{Shape: "git push", Count: 9},
		{Shape: "kubectl get pods", Count: 5},
		{Shape: "ls", Count: 4},
		{Shape: "git log", Count: 3},
	}
	want := []Count{{Shape: "kubectl get pods", Count: 5}, {Shape: "git log", Count: 3}}
	if got := ReadOnly(counts); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got := Rule("git log"); got != "Bash(git log:*)" {
		t.Fatalf("Rule = %q", got)
	}
}
