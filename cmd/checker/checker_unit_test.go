package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArgsFlagAndPositional(t *testing.T) {
	oldArgs := os.Args
	old := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = old
	}()

	// test with flags
	os.Args = []string{"cmd", "-file=./foo.log", "-hmac-key=abc", "-print"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg := parseArgs()
	require.Equal(t, "./foo.log", cfg.file)
	require.Equal(t, "abc", cfg.hmacKey)
	require.True(t, cfg.printLines)

	// test positional overrides flags when positional exists
	d := t.TempDir()
	p := filepath.Join(d, "pos.log")
	os.WriteFile(p, []byte("x"), 0600)
	os.Args = []string{"cmd", "-file=./foo.log", p}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg2 := parseArgs()
	require.Equal(t, p, cfg2.file)
}
