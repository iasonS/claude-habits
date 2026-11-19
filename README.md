# claude-habits

Shows which shell commands Claude Code runs again and again, so the repeated chores can become
a script, an alias or a permission rule.

It reads the session transcripts under `~/.claude/projects`, takes every command the assistant
ran through the Bash tool and reduces it to its shape: the program and its subcommand words,
without flags or arguments. `aws --profile dev logs start-query --query-string ...` becomes
`aws logs start-query`, and `kubectl -n web get pods -o wide` becomes `kubectl get pods`.
Compound commands are split on `&&`, `||`, `;`, `|` and newlines. Quoted strings, heredoc
bodies and comments are not read as commands.

## Run

    claude-habits                    # most frequent shapes, and in how many sessions they ran
    claude-habits sequences          # shapes that often run right after each other: script candidates
    claude-habits allowlist          # frequent read-only shapes as rules such as Bash(git log:*)
    claude-habits allowlist --json   # the same rules as a block to merge into settings.json

Flags work with every command:

    --since 30d      only calls from the last 30 days (12h and 90m work too)
    --project shop   only commands run in a directory whose path contains "shop"
    --top 50         rows to print, 0 for all (default 25)
    --all            include shell noise: cd, echo, grep, head, loop keywords and the like
    --json           print JSON
    --dir path       where the transcripts are (default ~/.claude/projects)

## What it does not do

- It counts what the assistant asked to run, including calls that were denied or failed.
- It sees only the Bash tool, not commands you typed yourself or other tools.
- Subcommand words are kept only for CLIs it knows, such as aws, gh, git, kubectl, docker,
  helm, terraform, go and npm. Any other program shows as its name alone.
- It reads commands; it does not run or expand them. Variables and functions stay as written.
- `allowlist` errs towards leaving things out, but a rule allows any arguments after its
  prefix, so read each one before you paste it. It never offers bare programs, script runners
  such as make, npm run or npx, or reads that can reveal credentials (secrets, tokens, config).
- It only reads local files and sends nothing anywhere.

## Build

Go runs in Docker, nothing to install:

    make test    # go vet and go test
    make build   # bin/claude-habits for this machine
