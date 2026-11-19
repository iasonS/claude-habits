// Package habits counts command shapes across Bash calls: which shapes repeat, which follow
// each other, and which are safe to allow without asking. It reads nothing itself.
package habits

import (
	"cmp"
	"slices"

	"github.com/iasonS/claude-habits/internal/shape"
	"github.com/iasonS/claude-habits/internal/shell"
)

// Call is one Bash call reduced to the shapes of its commands, in the order they run.
type Call struct {
	Session    string
	Transcript string
	Shapes     []string
}

// Count is how often a shape ran and in how many sessions.
type Count struct {
	Shape    string `json:"shape"`
	Count    int    `json:"count"`
	Sessions int    `json:"sessions"`
}

// Pair is a shape followed directly by another, counted like a Count.
type Pair struct {
	First    string `json:"first"`
	Then     string `json:"then"`
	Count    int    `json:"count"`
	Sessions int    `json:"sessions"`
}

// Shapes returns the shapes of the commands in one Bash command, leaving out shell noise
// unless withNoise is set.
func Shapes(command string, withNoise bool) []string {
	var shapes []string
	for _, words := range shell.Split(command) {
		s := shape.Of(words)
		if s == "" || (!withNoise && shape.Noise(s)) {
			continue
		}
		shapes = append(shapes, s)
	}
	return shapes
}

// Top counts every shape, most frequent first.
func Top(calls []Call) []Count {
	t := newTally[string]()
	for _, call := range calls {
		for _, s := range call.Shapes {
			t.add(s, call.Session)
		}
	}
	counts := make([]Count, 0, len(t.count))
	for s, n := range t.count {
		counts = append(counts, Count{Shape: s, Count: n, Sessions: len(t.sessions[s])})
	}
	// Most frequent first, then in most sessions, then by name.
	slices.SortFunc(counts, func(a, b Count) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(b.Sessions, a.Sessions), cmp.Compare(a.Shape, b.Shape))
	})
	return counts
}

type step struct{ first, then string }

// Sequences counts pairs of different shapes that run one after the other in the same
// transcript, within one call or across consecutive calls. Calls must be in transcript order.
func Sequences(calls []Call) []Pair {
	t := newTally[step]()
	previous := map[string]string{}
	for _, call := range calls {
		for _, s := range call.Shapes {
			last := previous[call.Transcript]
			previous[call.Transcript] = s
			if last != "" && last != s {
				t.add(step{last, s}, call.Session)
			}
		}
	}
	pairs := make([]Pair, 0, len(t.count))
	for st, n := range t.count {
		pairs = append(pairs, Pair{First: st.first, Then: st.then, Count: n, Sessions: len(t.sessions[st])})
	}
	slices.SortFunc(pairs, func(a, b Pair) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(b.Sessions, a.Sessions),
			cmp.Compare(a.First, b.First), cmp.Compare(a.Then, b.Then))
	})
	return pairs
}

// ReadOnly keeps the counts of shapes that only read, in their order.
func ReadOnly(counts []Count) []Count {
	var kept []Count
	for _, c := range counts {
		if shape.ReadOnly(c.Shape) {
			kept = append(kept, c)
		}
	}
	return kept
}

// Rule is the Claude Code permission rule that allows a shape with any arguments.
func Rule(s string) string {
	return "Bash(" + s + ":*)"
}

// tally counts how often each key occurs and in how many distinct sessions.
type tally[K comparable] struct {
	count    map[K]int
	sessions map[K]map[string]bool
}

func newTally[K comparable]() *tally[K] {
	return &tally[K]{count: map[K]int{}, sessions: map[K]map[string]bool{}}
}

func (t *tally[K]) add(key K, session string) {
	t.count[key]++
	if t.sessions[key] == nil {
		t.sessions[key] = map[string]bool{}
	}
	t.sessions[key][session] = true
}
