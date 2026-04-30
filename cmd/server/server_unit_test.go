package main

import (
	"flag"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"phonax.com/merkle/internal/server"
)

func TestParseFlags(t *testing.T) {
	// save and restore global state
	oldArgs := os.Args
	old := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = old
	}()

	os.Args = []string{"cmd", "-addr=:12345", "-backend=syslog", "-logfile=/tmp/x"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg := parseFlags()
	require.Equal(t, ":12345", cfg.addr)
	require.Equal(t, "syslog", cfg.backend)
	require.Equal(t, "/tmp/x", cfg.logfile)
}

func TestNewGRPCServerAndListener(t *testing.T) {
	s := newGRPCServer(nil)
	require.NotNil(t, s)

	lis, err := createListener("127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
}

func TestRegisterAndServeStartsAndStops(t *testing.T) {
	// create listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

	s := newGRPCServer(nil)

	// create a simple file-based service in temp dir
	p := filepath.Join(t.TempDir(), "protected.log")
	svc, err := server.NewService(server.Config{Backend: "file", LogFile: p})
	require.NoError(t, err)
	defer svc.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- registerAndServe(s, svc, lis, lis.Addr().String())
	}()

	// give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// stop the server gracefully
	s.GracefulStop()

	// wait for registerAndServe to return
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop in time")
	}
}
