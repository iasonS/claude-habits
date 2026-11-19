package shape

import "strings"

// noise are programs that are shell syntax, builtins, or plumbing around the real command:
// text filters, file basics and output.
var noise = set(
	"!", ".", ":", "[", "[[", "{", "}", "alias", "break", "case", "cd", "command", "continue",
	"declare", "do", "done", "echo", "elif", "else", "esac", "eval", "exit", "export", "false", "fi",
	"for", "function", "if", "in", "local", "popd", "print", "printf", "pushd", "pwd", "read", "return",
	"select", "set", "shift", "source", "test", "then", "trap", "true", "type", "unset", "until",
	"wait", "which", "while",
	"awk", "basename", "cat", "chmod", "column", "cp", "cut", "date", "dirname", "find", "grep",
	"head", "jq", "less", "ls", "mkdir", "mv", "nl", "paste", "readlink", "realpath", "rg", "rm",
	"sed", "seq", "sleep", "sort", "stat", "tail", "tee", "touch", "tr", "uniq", "wc", "xargs",
)

// readVerbs are operations that only read. An operation also counts when it starts with one
// of these followed by a dash, as in "describe-log-groups" or "ls-files".
var readVerbs = set(
	"blame", "describe", "diff", "explain", "get", "history", "info", "inspect", "list", "log",
	"logs", "ls", "ps", "show", "status", "top", "version", "view", "whoami",
)

// sensitive marks read-only operations that should still ask first, because they reveal
// credentials or write the object they read to a local file.
var sensitive = []string{
	"auth", "config", "credential", "get-object", "login", "parameter", "password", "secret", "token",
}

// runners run scripts the user defined, so their subcommand words can name anything.
var runners = set("make", "npm run", "npx", "pnpm run", "uv run", "yarn")

// Noise reports whether a shape is shell syntax or plumbing rather than a chore of its own.
func Noise(shape string) bool {
	program, _, _ := strings.Cut(shape, " ")
	return noise[program]
}

// ReadOnly reports whether every command of this shape only reads, so a permission rule that
// allows the shape with any arguments is safe. It errs towards false.
func ReadOnly(shape string) bool {
	words := strings.Fields(shape)
	if len(words) < 2 || wanted(words) != len(words) || isSensitive(shape) || isRunner(words) {
		return false
	}
	return isReadVerb(operation(words))
}

// operation is the word of a complete shape that says what it does: the last word, or the
// one before it when the last word names the kind of object, as in "kubectl get pods".
func operation(words []string) string {
	last := len(words) - 1
	if objects[strings.Join(words[:last], " ")] {
		return words[last-1]
	}
	return words[last]
}

func isReadVerb(word string) bool {
	verb, _, _ := strings.Cut(word, "-")
	return readVerbs[verb]
}

func isRunner(words []string) bool {
	return runners[words[0]] || runners[words[0]+" "+words[1]]
}

func isSensitive(shape string) bool {
	for _, word := range sensitive {
		if strings.Contains(shape, word) {
			return true
		}
	}
	return false
}
