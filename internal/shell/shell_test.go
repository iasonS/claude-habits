package shell

import (
	"reflect"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   [][]string
	}{
		{
			name:   "operators",
			script: "a 1 && b || c; d | e & f |& g",
			want:   [][]string{{"a", "1"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}},
		},
		{
			name:   "quotes keep operators inside one word",
			script: `echo "a && b" 'c | d; e' x\;y`,
			want:   [][]string{{"echo", "a && b", "c | d; e", "x;y"}},
		},
		{
			name:   "newlines separate commands",
			script: "git status\ngit log --oneline\n",
			want:   [][]string{{"git", "status"}, {"git", "log", "--oneline"}},
		},
		{
			name:   "line continuation joins lines",
			script: "aws logs \\\n  start-query --limit 5",
			want:   [][]string{{"aws", "logs", "start-query", "--limit", "5"}},
		},
		{
			name:   "heredoc body is skipped",
			script: "cat <<'EOF' > job.py\nimport os\nfor x in y: print(x) && z\nEOF\npython3 job.py",
			want:   [][]string{{"cat"}, {"python3", "job.py"}},
		},
		{
			name:   "heredoc with dash strips tabs from the delimiter line",
			script: "psql <<-SQL\n\tselect 1; drop table t;\n\tSQL\nls",
			want:   [][]string{{"psql"}, {"ls"}},
		},
		{
			name:   "heredoc feeding a pipe",
			script: "cat <<EOF | kubectl apply -f -\nkind: Pod\nEOF",
			want:   [][]string{{"cat"}, {"kubectl", "apply", "-f", "-"}},
		},
		{
			name:   "heredoc inside a substitution inside double quotes",
			script: "git commit -m \"$(cat <<'EOF'\nIt's done; (really)\nEOF\n)\" && git push",
			want:   [][]string{{"cat"}, {"git", "commit", "-m", "$(...)"}, {"git", "push"}},
		},
		{
			name:   "command substitution runs first",
			script: "ID=$(aws logs start-query --name q) && aws logs get-query-results --id \"$ID\"",
			want: [][]string{
				{"aws", "logs", "start-query", "--name", "q"},
				{"ID=$(...)"},
				{"aws", "logs", "get-query-results", "--id", "$ID"},
			},
		},
		{
			name:   "arithmetic is not a command",
			script: "echo $((1 + 2))",
			want:   [][]string{{"echo", "$(...)"}},
		},
		{
			name:   "backquotes",
			script: "echo `date +%s`",
			want:   [][]string{{"date", "+%s"}, {"echo", "$(...)"}},
		},
		{
			name:   "process substitution",
			script: "diff <(sort a) <(sort b)",
			want:   [][]string{{"sort", "a"}, {"sort", "b"}, {"diff"}},
		},
		{
			name:   "redirections are dropped",
			script: "kubectl logs p 2>&1 >/dev/null < in &>> all >&2 <<< \"text\"",
			want:   [][]string{{"kubectl", "logs", "p"}},
		},
		{
			name:   "comments are dropped",
			script: "# check first\nls -la # list\necho a#b",
			want:   [][]string{{"ls", "-la"}, {"echo", "a#b"}},
		},
		{
			name:   "subshell",
			script: "(cd repo && make test) | tee log",
			want:   [][]string{{"cd", "repo"}, {"make", "test"}, {"tee", "log"}},
		},
		{
			name:   "loop",
			script: "for f in *.go; do gofmt -l \"$f\"; done",
			want:   [][]string{{"for", "f", "in", "*.go"}, {"do", "gofmt", "-l", "$f"}, {"done"}},
		},
		{
			name:   "case patterns are not commands",
			script: "case \"$f\" in\n  *.go|*.mod) go vet ;;\n  F) make ;;\n  *) ls ;;\nesac",
			want:   [][]string{{"case", "$f", "in"}, {"go", "vet"}, {"make"}, {"ls"}, {"esac"}},
		},
		{
			name:   "case on one line",
			script: "case $x in a) git pull;; b|c) git push;; esac; ls",
			want:   [][]string{{"case", "$x", "in"}, {"git", "pull"}, {"git", "push"}, {"esac"}, {"ls"}},
		},
		{
			name:   "parameter expansion may hold blanks",
			script: "name=${pair%% *}; echo ${#name} \"${pair#* }\"",
			want:   [][]string{{"name=${pair%% *}"}, {"echo", "${#name}", "${pair#* }"}},
		},
		{
			name:   "array assignment",
			script: "hosts=([a]=one [b]=two) && ssh x",
			want:   [][]string{{"hosts=([a]=one [b]=two)"}, {"ssh", "x"}},
		},
		{
			name:   "unterminated quote does not hang",
			script: `echo "abc`,
			want:   [][]string{{"echo", "abc"}},
		},
		{
			name:   "unterminated substitution and backquote do not hang",
			script: "echo $(ls `pwd \\",
			want:   [][]string{{"pwd"}, {"ls", "$(...)"}, {"echo", "$(...)"}},
		},
		{
			name:   "empty",
			script: " \n ; && \n",
			want:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Split(test.script)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Split(%q)\n got %q\nwant %q", test.script, got, test.want)
			}
		})
	}
}

func TestSplitNeverPanics(t *testing.T) {
	for _, script := range []string{"\\", "`", "$(", "$((", "'", "\"", "<<", "<<<", "2>", "&>", "<(", "\"$(", "\"\\", "cat <<EOF", "${", "a=(", ")", "case x in"} {
		Split(script)
	}
}
