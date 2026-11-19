// Package shell splits a shell script into its simple commands. It understands enough of
// the shell to keep quoted strings, heredoc bodies and comments from being read as commands;
// it does not run or expand anything.
package shell

import "strings"

const substitution = "$(...)"

type heredoc struct {
	delimiter string
	stripTabs bool
}

type scanner struct {
	src      string
	pos      int
	heredocs []heredoc // waiting for the next newline, where their bodies start
	pattern  bool      // a case pattern such as "*.go|*.md)" comes next
	commands [][]string
}

// Split returns the simple commands of a script in the order they run, each as its words
// with quotes removed and redirections dropped. Commands inside $(...), backquotes and
// <(...) come before the command that uses their output.
func Split(script string) [][]string {
	s := &scanner{src: script}
	s.list(false)
	return s.commands
}

// list reads commands until the end of the script or, when nested, the ")" that closes it.
func (s *scanner) list(nested bool) {
	var words []string
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case isBlank(c):
			s.pos++
		case s.hasPrefix("\\\n"):
			s.pos += 2
		case c == '\n':
			s.pos++
			words = s.finish(words)
			s.skipHeredocs()
		case c == '#':
			s.skipComment()
		case s.pattern:
			s.skipPattern()
		case c == ')':
			s.pos++
			words = s.finish(words)
			if nested {
				return
			}
		case c == '(':
			s.pos++
			words = s.finish(words)
			s.list(true)
		case s.hasPrefix("<(") || s.hasPrefix(">("):
			s.pos += 2
			s.list(true)
		case c == '<' || c == '>' || s.hasPrefix("&>") || s.atNumberedRedirect():
			s.redirect()
		case c == ';' || c == '&' || c == '|':
			s.pattern = s.hasPrefix(";;") || s.hasPrefix(";&")
			s.operator()
			words = s.finish(words)
		default:
			if word := s.word(); word != "" {
				words = append(words, word)
			}
			if len(words) == 3 && words[0] == "case" && words[2] == "in" {
				words = s.finish(words)
				s.pattern = true
			}
		}
	}
	s.finish(words)
}

func (s *scanner) finish(words []string) []string {
	if len(words) > 0 {
		s.commands = append(s.commands, words)
	}
	return nil
}

// operator skips a command separator: ; ;; & && | || |&
func (s *scanner) operator() {
	s.pos++
	if s.pos < len(s.src) && strings.IndexByte(";&|", s.src[s.pos]) >= 0 {
		s.pos++
	}
}

// skipPattern skips a case pattern up to its ")", unless the case statement ends instead.
func (s *scanner) skipPattern() {
	s.pattern = false
	if s.hasPrefix("esac") {
		return
	}
	end := strings.IndexByte(s.src[s.pos:], ')')
	if end < 0 {
		s.pos = len(s.src)
		return
	}
	s.pos += end + 1
}

// atNumberedRedirect reports whether the scanner is at a redirection of a numbered file
// descriptor, such as the 2 of 2>&1.
func (s *scanner) atNumberedRedirect() bool {
	end := s.pos
	for end < len(s.src) && isDigit(s.src[end]) {
		end++
	}
	return end > s.pos && end < len(s.src) && (s.src[end] == '<' || s.src[end] == '>')
}

// redirect skips a redirection and its target: > out, 2>&1, &>/dev/null, <<<"text", <<EOF.
func (s *scanner) redirect() {
	for s.pos < len(s.src) && isDigit(s.src[s.pos]) {
		s.pos++
	}
	if s.hasPrefix("<<<") {
		s.pos += 3
		s.target()
		return
	}
	if s.hasPrefix("<<") {
		s.pos += 2
		stripTabs := s.hasPrefix("-")
		if stripTabs {
			s.pos++
		}
		s.heredocs = append(s.heredocs, heredoc{delimiter: s.target(), stripTabs: stripTabs})
		return
	}
	s.pos++
	for s.pos < len(s.src) && strings.IndexByte("<>&|-", s.src[s.pos]) >= 0 {
		s.pos++
	}
	s.target()
}

func (s *scanner) target() string {
	for s.pos < len(s.src) && isBlank(s.src[s.pos]) {
		s.pos++
	}
	return s.word()
}

// skipHeredocs skips the bodies of the heredocs started on the line that just ended.
func (s *scanner) skipHeredocs() {
	for _, h := range s.heredocs {
		s.skipHeredoc(h)
	}
	s.heredocs = nil
}

func (s *scanner) skipHeredoc(h heredoc) {
	for s.pos < len(s.src) {
		line := s.line()
		if h.stripTabs {
			line = strings.TrimLeft(line, "\t")
		}
		if line == h.delimiter {
			return
		}
	}
}

// line returns the rest of the current line and moves past its newline.
func (s *scanner) line() string {
	end := strings.IndexByte(s.src[s.pos:], '\n')
	if end < 0 {
		line := s.src[s.pos:]
		s.pos = len(s.src)
		return line
	}
	line := s.src[s.pos : s.pos+end]
	s.pos += end + 1
	return line
}

func (s *scanner) skipComment() {
	end := strings.IndexByte(s.src[s.pos:], '\n')
	if end < 0 {
		s.pos = len(s.src)
		return
	}
	s.pos += end
}

// word reads one word up to the next unquoted blank or operator and returns it without quotes.
func (s *scanner) word() string {
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '(' && strings.HasSuffix(b.String(), "="):
			b.WriteString(s.balanced('(', ')')) // an array, as in names=(a b)
		case isBlank(c) || isOperator(c):
			return b.String()
		case c == '\\':
			s.escaped(&b)
		case c == '\'':
			b.WriteString(s.singleQuoted())
		case c == '"':
			b.WriteString(s.doubleQuoted())
		case c == '`':
			s.backquoted()
			b.WriteString(substitution)
		case s.hasPrefix("$("):
			s.substitution()
			b.WriteString(substitution)
		case s.hasPrefix("${"):
			s.pos++
			b.WriteString("$" + s.balanced('{', '}')) // may hold blanks, as in ${name% *}
		default:
			b.WriteByte(c)
			s.pos++
		}
	}
	return b.String()
}

// escaped writes the character after a backslash; a backslash before a newline joins lines.
func (s *scanner) escaped(b *strings.Builder) {
	s.pos++
	if s.pos >= len(s.src) {
		return
	}
	if s.src[s.pos] != '\n' {
		b.WriteByte(s.src[s.pos])
	}
	s.pos++
}

func (s *scanner) singleQuoted() string {
	s.pos++
	end := strings.IndexByte(s.src[s.pos:], '\'')
	if end < 0 {
		text := s.src[s.pos:]
		s.pos = len(s.src)
		return text
	}
	text := s.src[s.pos : s.pos+end]
	s.pos += end + 1
	return text
}

func (s *scanner) doubleQuoted() string {
	var b strings.Builder
	s.pos++
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '"':
			s.pos++
			return b.String()
		case c == '\\' && s.pos+1 < len(s.src) && strings.IndexByte("\"\\$`\n", s.src[s.pos+1]) >= 0:
			s.escaped(&b)
		case c == '`':
			s.backquoted()
			b.WriteString(substitution)
		case s.hasPrefix("$("):
			s.substitution()
			b.WriteString(substitution)
		default:
			b.WriteByte(c)
			s.pos++
		}
	}
	return b.String()
}

// substitution reads $(...) and records the commands inside it. $((...)) is arithmetic and
// holds no commands.
func (s *scanner) substitution() {
	s.pos += 2
	if s.hasPrefix("(") {
		s.balanced('(', ')')
		if s.hasPrefix(")") {
			s.pos++
		}
		return
	}
	s.list(true)
}

// balanced reads from an opening bracket to just after the bracket that closes it.
func (s *scanner) balanced(opening, closing byte) string {
	start := s.pos
	depth := 0
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		s.pos++
		if c == opening {
			depth++
		}
		if c == closing {
			depth--
		}
		if depth == 0 {
			break
		}
	}
	return s.src[start:s.pos]
}

// backquoted reads `...` and records the commands inside it.
func (s *scanner) backquoted() {
	s.pos++
	start := s.pos
	for s.pos < len(s.src) && s.src[s.pos] != '`' {
		if s.src[s.pos] == '\\' && s.pos+1 < len(s.src) {
			s.pos++
		}
		s.pos++
	}
	inner := s.src[start:s.pos]
	if s.pos < len(s.src) {
		s.pos++
	}
	s.commands = append(s.commands, Split(inner)...)
}

func (s *scanner) hasPrefix(prefix string) bool {
	return strings.HasPrefix(s.src[s.pos:], prefix)
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r'
}

func isOperator(c byte) bool {
	return strings.IndexByte(";&|()<>\n", c) >= 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
