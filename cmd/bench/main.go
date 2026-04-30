package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"phonax.com/merkle/internal/checker"
	"phonax.com/merkle/internal/server"
	"phonax.com/merkle/proto"
)

func main() {
	duration := flag.Int("duration", 60, "duration in seconds")
	workers := flag.Int("workers", 0, "number of concurrent workers (default: 2*CPUs)")
	outfile := flag.String("out", "", "path to write produced logfile (if empty kept in temp dir and printed)")
	hmacKey := flag.String("hmac-key", os.Getenv("MERKLE_HMAC_KEY"), "HMAC key for checker")
	flag.Parse()

	if *workers == 0 {
		*workers = runtime.NumCPU() * 2
	}

	// Start gRPC server listening on random local port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr := lis.Addr().String()

	// create service with file backend in temp dir
	outPath := *outfile
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), fmt.Sprintf("merkle-bench-%d.log", time.Now().Unix()))
	}
	cfg := server.Config{Backend: "file", LogFile: outPath}
	svc, err := server.NewService(cfg)
	if err != nil {
		panic(err)
	}
	defer svc.Close()

	s := grpc.NewServer()
	proto.RegisterLoggerServer(s, svc)
	go func() { s.Serve(lis) }()

	// create client
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := proto.NewLoggerClient(conn)

	var success uint64
	var failures uint64
	var total uint64

	stop := time.Now().Add(time.Duration(*duration) * time.Second)
	var wg sync.WaitGroup
	wg.Add(*workers)
	for i := 0; i < *workers; i++ {
		go func(id int) {
			defer wg.Done()
			for time.Now().Before(stop) {
				atomic.AddUint64(&total, 1)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := client.Write(ctx, &proto.LogRequest{Application: "bench", Level: "info", Message: "ping", Fields: map[string]string{"worker": strconv.Itoa(id)}})
				cancel()
				if err != nil {
					atomic.AddUint64(&failures, 1)
					continue
				}
				atomic.AddUint64(&success, 1)
			}
		}(i)
	}
	wg.Wait()

	succ := atomic.LoadUint64(&success)
	fail := atomic.LoadUint64(&failures)
	tot := atomic.LoadUint64(&total)

	// run checker
	err = checker.CheckFile(outPath, *hmacKey)
	checkerOk := err == nil
	if !checkerOk {
		fmt.Fprintf(os.Stderr, "checker error: %v\n", err)
	}

	fmt.Printf("RESULT duration=%ds workers=%d total=%d success=%d failures=%d checker_ok=%v logfile=%s\n", *duration, *workers, tot, succ, fail, checkerOk, outPath)
}
