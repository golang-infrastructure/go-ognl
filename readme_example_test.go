package ognl_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const readmeExampleModule = "github.com/golang-infrastructure/go-ognl"

func TestREADMEExample(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}

	source := markdownFenceAfter(t, string(readme), "## 完整示例", "go")
	want := markdownFenceAfter(t, string(readme), "输出：", "text")

	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	dir := t.TempDir()
	goMod := fmt.Sprintf("module readme-example\n\ngo 1.18\n\nrequire %s v0.0.0\n\nreplace %s => %s\n", readmeExampleModule, readmeExampleModule, root)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatalf("write example go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write README example: %v", err)
	}

	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run README example: %v\n%s", err, output)
	}
	if got := string(output); got != want {
		t.Fatalf("README example output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func markdownFenceAfter(t *testing.T, markdown, heading, language string) string {
	t.Helper()

	headingIndex := strings.Index(markdown, heading)
	if headingIndex < 0 {
		t.Fatalf("README heading %q not found", heading)
	}
	marker := "```" + language + "\n"
	content := markdown[headingIndex+len(heading):]
	start := strings.Index(content, marker)
	if start < 0 {
		t.Fatalf("README %s fence after %q not found", language, heading)
	}
	content = content[start+len(marker):]
	end := strings.Index(content, "\n```")
	if end < 0 {
		t.Fatalf("README %s fence after %q is not closed", language, heading)
	}
	return content[:end] + "\n"
}
