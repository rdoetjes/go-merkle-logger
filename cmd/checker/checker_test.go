package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func buildAndRun(t *testing.T, pkg string, args ...string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", out)
	cmd.Dir = "."
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s failed: %v\n%s", pkg, err, string(outb))
	}
	run := exec.Command(out, args...)
	if outb, err := run.CombinedOutput(); err != nil {
		t.Fatalf("run %s failed: %v\n%s", pkg, err, string(outb))
	}
}

func TestCheckerHelp(t *testing.T) {
	buildAndRun(t, "cmd/checker", "-h")
}
