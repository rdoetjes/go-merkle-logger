package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDialOptionsNoCA(t *testing.T) {
	opts, err := buildDialOptions("")
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestLoadDialOptionsWithBadCA(t *testing.T) {
	// write temp bad ca file
	f := t.TempDir() + "/bad.pem"
	os.WriteFile(f, []byte("not a cert"), 0600)
	_, err := buildDialOptions(f)
	require.Error(t, err)
}
