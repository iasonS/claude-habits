package shape

import (
	"strings"
	"testing"
)

func TestOf(t *testing.T) {
	tests := map[string]string{
		"aws --profile dev logs start-query --log-group-name app --query-string x": "aws logs start-query",
		"aws s3 ls s3://bucket/path":                      "aws s3 ls",
		"kubectl -n prod get pods -o wide":                "kubectl get pods",
		"kubectl get pod/web-1":                           "kubectl get",
		"kubectl logs web-1 -c app --since 1h":            "kubectl logs",
		"kubectl rollout restart deployment/web":          "kubectl rollout restart",
		"gh pr view 12 --json title":                      "gh pr view",
		"gh api repos/owner/name/pulls":                   "gh api",
		"git show HEAD:README.md":                         "git show",
		"git -C ../other log --oneline -5":                "git log",
		"git stash list":                                  "git stash list",
		"git stash":                                       "git stash",
		"docker compose -f dev.yml up -d":                 "docker compose up",
		"FOO=1 BAR=x make test":                           "make test",
		"env AWS_PROFILE=dev aws sts get-caller-identity": "aws sts get-caller-identity",
		"sudo -u root systemctl restart web":              "systemctl restart",
		"timeout 30 kubectl get nodes":                    "kubectl get nodes",
		"xargs -I {} kubectl describe pod {}":             "kubectl describe pod",
		"rtk proxy git status":                            "git status",
		"do kubectl get pods":                             "kubectl get pods",
		"then aws s3 ls":                                  "aws s3 ls",
		"python3 script.py --fast":                        "python3",
		"./scripts/release.sh v1":                         "./scripts/release.sh",
		"grep -rn needle .":                               "grep",
		"X=$(...)":                                        "",
		"time":                                            "",
	}
	for command, want := range tests {
		if got := Of(strings.Fields(command)); got != want {
			t.Errorf("Of(%q) = %q, want %q", command, got, want)
		}
	}
}

func TestNoise(t *testing.T) {
	for _, shape := range []string{"cd", "echo", "head", "for", "done", "then", "fi", "grep", "["} {
		if !Noise(shape) {
			t.Errorf("%q is not noise", shape)
		}
	}
	for _, shape := range []string{"git status", "aws logs start-query", "curl", "python3"} {
		if Noise(shape) {
			t.Errorf("%q is noise", shape)
		}
	}
}

func TestReadOnly(t *testing.T) {
	readOnly := []string{
		"git status", "git log", "git diff", "git show", "git ls-files", "git stash list",
		"kubectl get pods", "kubectl logs", "kubectl rollout status", "kubectl top nodes",
		"aws logs describe-log-groups", "aws logs get-query-results", "aws s3 ls",
		"gh pr view", "gh run list", "docker ps", "docker compose logs",
	}
	for _, shape := range readOnly {
		if !ReadOnly(shape) {
			t.Errorf("%q is not read-only", shape)
		}
	}

	writes := []string{
		"git push", "git stash", "git branch", "kubectl delete pods", "kubectl rollout restart",
		"kubectl apply", "kubectl get", "aws logs", "aws logs delete-log-group", "aws logs tag-resource",
		"aws logs start-query", "aws s3 cp", "gh pr merge", "gh api", "docker run", "terraform plan",
		"ls", "curl",
	}
	for _, shape := range writes {
		if ReadOnly(shape) {
			t.Errorf("%q is read-only", shape)
		}
	}

	revealing := []string{
		"aws secretsmanager get-secret-value", "aws ssm get-parameter", "aws ecr get-login-password",
		"aws s3api get-object", "kubectl get secrets", "kubectl config view", "gh auth status",
	}
	for _, shape := range revealing {
		if ReadOnly(shape) {
			t.Errorf("%q is offered although it reveals credentials or writes a file", shape)
		}
	}

	for _, shape := range []string{"make logs", "npm run status", "yarn list", "npx view"} {
		if ReadOnly(shape) {
			t.Errorf("%q runs a user-defined script but is read-only", shape)
		}
	}
}
