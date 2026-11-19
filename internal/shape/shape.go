// Package shape reduces a command to its shape: the program plus the subcommand words that
// say what it does, such as "aws logs start-query" or "git show", without flags or arguments.
package shape

import (
	"regexp"
	"strings"
)

// depth is how many subcommand words follow the program. A program not listed keeps none.
var depth = map[string]int{
	"aws": 2, "gh": 2,
	"brew": 1, "cargo": 1, "docker": 1, "flux": 1, "git": 1, "go": 1, "helm": 1, "kubectl": 1,
	"make": 1, "npm": 1, "npx": 1, "pip": 1, "pip3": 1, "pnpm": 1, "podman": 1, "systemctl": 1,
	"terraform": 1, "tofu": 1, "uv": 1, "yarn": 1,
}

// groups are commands whose next word names the operation, as in "docker compose up".
var groups = set(
	"docker buildx", "docker compose", "docker container", "docker context", "docker image",
	"docker network", "docker system", "docker volume",
	"git remote", "git stash", "git submodule", "git worktree",
	"go mod", "helm dependency", "helm repo",
	"kubectl auth", "kubectl config", "kubectl rollout",
	"npm run", "pnpm run", "terraform state", "terraform workspace", "tofu state", "tofu workspace",
	"uv pip", "uv run",
)

// objects are commands whose next word names the kind of object they act on, as in
// "kubectl get pods".
var objects = set(
	"flux get", "flux reconcile", "flux resume", "flux suspend", "helm get", "helm show",
	"kubectl annotate", "kubectl create", "kubectl delete", "kubectl describe", "kubectl edit",
	"kubectl explain", "kubectl get", "kubectl label", "kubectl patch", "kubectl scale", "kubectl top",
)

// valueFlags take the next word as their value, so that word is not a subcommand, as in
// "kubectl -n prod get pods".
var valueFlags = set(
	"-C", "-c", "-f", "-l", "-n", "-o", "-p", "-R",
	"--context", "--file", "--kubeconfig", "--namespace", "--output", "--profile", "--region",
	"--repo", "--selector",
)

// prefixes come before the program and are not part of the shape.
var prefixes = set("!", "{", "do", "elif", "else", "if", "then", "until", "while")

// wrappers run the command that follows them.
var wrappers = []string{
	"env", "exec", "nice", "nohup", "rtk proxy", "sudo", "time", "timeout", "watch", "xargs",
}

// wrapperValueFlags are wrapper flags that take a value, as in "sudo -u root" or "xargs -n 1".
var wrapperValueFlags = set("-I", "-L", "-P", "-d", "-k", "-n", "-s", "-u")

var (
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\+?=`)
	subcommand = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Of returns the shape of one simple command given as its words, or "" when the words hold
// no command, for example only variable assignments.
func Of(words []string) string {
	words = skipPrefixes(words)
	if len(words) == 0 {
		return ""
	}
	shape := []string{words[0]}
	rest := words[1:]
	for len(shape) < wanted(shape) {
		word, remaining, found := nextSubcommand(rest)
		if !found {
			break
		}
		shape = append(shape, word)
		rest = remaining
	}
	return strings.Join(shape, " ")
}

// wanted is how many words a complete shape starting with these words has. It is more than
// len(words) while subcommand words are still missing.
func wanted(words []string) int {
	n := 1 + depth[words[0]]
	for n <= len(words) && isNested(words[:n]) {
		n++
	}
	return n
}

func isNested(words []string) bool {
	command := strings.Join(words, " ")
	return groups[command] || objects[command]
}

// nextSubcommand skips flags and returns the next word if it looks like a subcommand.
func nextSubcommand(words []string) (string, []string, bool) {
	for len(words) > 0 {
		word := words[0]
		switch {
		case valueFlags[word]:
			words = words[min(2, len(words)):]
		case strings.HasPrefix(word, "-"):
			words = words[1:]
		case subcommand.MatchString(word):
			return word, words[1:], true
		default:
			return "", nil, false
		}
	}
	return "", nil, false
}

// skipPrefixes drops what comes before the program: assignments, keywords and wrappers.
func skipPrefixes(words []string) []string {
	for len(words) > 0 {
		if assignment.MatchString(words[0]) || prefixes[words[0]] {
			words = words[1:]
			continue
		}
		rest, wrapped := skipWrapper(words)
		if !wrapped {
			return words
		}
		words = rest
	}
	return words
}

// skipWrapper drops a wrapper with its flags and numbers, as in "timeout -s KILL 30".
func skipWrapper(words []string) ([]string, bool) {
	for _, wrapper := range wrappers {
		name := strings.Fields(wrapper)
		if len(words) < len(name) || strings.Join(words[:len(name)], " ") != wrapper {
			continue
		}
		return skipWrapperOptions(words[len(name):]), true
	}
	return words, false
}

func skipWrapperOptions(words []string) []string {
	for len(words) > 0 {
		word := words[0]
		switch {
		case wrapperValueFlags[word]:
			words = words[min(2, len(words)):]
		case strings.HasPrefix(word, "-") || startsWithDigit(word):
			words = words[1:]
		default:
			return words
		}
	}
	return words
}

func startsWithDigit(word string) bool {
	return word != "" && word[0] >= '0' && word[0] <= '9'
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}
