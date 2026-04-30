package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClientHelp(t *testing.T) {
	out := filepath.Join(t.TempDir(), "client")
	cmd := exec.Command("go", "build", "-o", out)
	cmd.Dir = "."
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build client failed: %v\n%s", err, string(outb))
	}
	run := exec.Command(out, "-h")
	if outb, err := run.CombinedOutput(); err != nil {
		t.Fatalf("run client failed: %v\n%s", err, string(outb))
	}
}
