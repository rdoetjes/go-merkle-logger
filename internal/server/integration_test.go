package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"phonax.com/merkle/internal/checker"
	"phonax.com/merkle/proto"
)

// TestIntegrationRate performs a simple throughput test against an in-process server.
// By default this runs for 60s. To run a shorter test in CI, run `go test -short` to skip it.
// You can override duration in seconds using the MERKLE_BENCH_DURATION environment variable.
func TestIntegrationRate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration rate test in short mode")
	}

	// duration (seconds)
	durSec := 60
	if v := getenvInt("MERKLE_BENCH_DURATION", 0); v > 0 {
		durSec = v
	}
	duration := time.Duration(durSec) * time.Second

	// Start gRPC server listening on a random local port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()

	// create service with file backend in temp dir. Use a preserved temp filename so tests can inspect it later.
	logfile := filepath.Join(os.TempDir(), fmt.Sprintf("merkle-integration-%d.log", time.Now().UnixNano()))
	svc, err := NewService(Config{Backend: "file", LogFile: logfile})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	// Print logfile path both to the test logger and to stdout so it can be captured easily.
	t.Logf("logfile: %s", logfile)
	fmt.Printf("LOGFILE:%s\n", logfile)

	s := grpc.NewServer()
	proto.RegisterLoggerServer(s, svc)

	srvErrC := make(chan error, 1)
	go func() {
		srvErrC <- s.Serve(lis)
	}()
	// give server a moment
	t.Logf("server listening on %s", addr)

	// create client connection
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := proto.NewLoggerClient(conn)

	// benchmark parameters
	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 1 {
		numWorkers = 1
	}
	var success uint64
	var failures uint64
	var total uint64

	t.Logf("running for %s with %d workers", duration.String(), numWorkers)

	stop := time.Now().Add(duration)
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			for time.Now().Before(stop) {
				atomic.AddUint64(&total, 1)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := client.Write(ctx, &proto.LogRequest{Application: "bench", Level: "info", Message: "ping", Fields: map[string]string{"worker": strconv.Itoa(id)}})
				cancel()
				if err != nil {
					atomic.AddUint64(&failures, 1)
					// back off a tiny bit
					time.Sleep(10 * time.Millisecond)
					continue
				}
				atomic.AddUint64(&success, 1)
			}
		}(i)
	}

	// periodic logging
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			succ := atomic.LoadUint64(&success)
			tot := atomic.LoadUint64(&total)
			fail := atomic.LoadUint64(&failures)
			t.Logf("elapsed: %s total=%d success=%d failures=%d", time.Since(time.Now().Add(-duration)).Truncate(time.Second), tot, succ, fail)
		}
	}()

	wg.Wait()
	ticker.Stop()

	succ := atomic.LoadUint64(&success)
	fail := atomic.LoadUint64(&failures)
	tot := atomic.LoadUint64(&total)

	// shutdown server
	s.GracefulStop()
	select {
	case err := <-srvErrC:
		if err != nil && err != grpc.ErrServerStopped {
			t.Logf("server returned: %v", err)
		}
	default:
	}

	resLine := fmt.Sprintf("RESULT duration=%s workers=%d total=%d success=%d failures=%d\n", duration, numWorkers, tot, succ, fail)
	t.Logf("done: %s", resLine)
	// run the checker on the produced logfile
	if err := checker.CheckFile(logfile, os.Getenv("MERKLE_HMAC_KEY")); err != nil {
		t.Errorf("checker failed: %v", err)
	} else {
		t.Logf("checker: OK")
	}

	// Optionally preserve logfile into project directory for inspection when MERKLE_PRESERVE_LOG=1
	if os.Getenv("MERKLE_PRESERVE_LOG") == "1" {
		dst := filepath.Join(".", filepath.Base(logfile))
		if err := copyFile(logfile, dst); err != nil {
			t.Logf("failed to copy logfile to %s: %v", dst, err)
		} else {
			fmt.Printf("PRESERVED_LOG:%s\n", dst)
		}
	}

	// Print to stdout too for CI-friendly parsing
	fmt.Print(resLine)
}

func copyFile(src, dst string) error {
	sr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sr.Close()
	dw, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dw.Close()
	if _, err := io.Copy(dw, sr); err != nil {
		return err
	}
	return nil
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
