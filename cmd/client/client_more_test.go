package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDialOptionsBadCA(t *testing.T) {
	// create bad CA file
	f := t.TempDir() + "/bad.pem"
	os.WriteFile(f, []byte("not a pem"), 0600)
	_, err := buildDialOptions(f)
	require.Error(t, err)
}

func TestParseClientFlagsDefaults(t *testing.T) {
	oldArgs := os.Args
	old := flag.CommandLine
	defer func() { os.Args = oldArgs; flag.CommandLine = old }()

	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	addr, ca := parseClientFlags()
	require.Equal(t, "localhost:8443", addr)
	require.Equal(t, "", ca)
}
