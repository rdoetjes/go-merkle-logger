package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"phonax.com/merkle/merklelog"
)

// CLI config parsed from flags/args
type config struct {
	file       string
	hmacKey    string
	printLines bool
}

func parseArgs() config {
	file := flag.String("file", "./protected.log", "path to protected log file")
	hmacKey := flag.String("hmac-key", "", "HMAC key for signature verification (base64 or raw)")
	printEntries := flag.Bool("print", false, "print entries as parsed")
	flag.Parse()

	// Choose the first existing non-flag argument as filename if provided
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if _, err := os.Stat(a); err == nil {
			*file = a
			break
		}
	}

	// If flag not provided, fallback to env MERKLE_HMAC_KEY (raw or base64). This mirrors server behavior.
	if *hmacKey == "" {
		if env := os.Getenv("MERKLE_HMAC_KEY"); env != "" {
			*hmacKey = env
		}
	}

	return config{file: *file, hmacKey: *hmacKey, printLines: *printEntries}
}

func printUsageAndExit() {
	fmt.Fprintf(os.Stderr, "Usage: merkle-checker [flags] [logfile]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func printFileInfo(path string) {
	abs, _ := filepath.Abs(path)
	fmt.Printf("Checking file: %s\n", abs)
}

func main() {
	cfg := parseArgs()
	if cfg.file == "" {
		printUsageAndExit()
	}
	printFileInfo(cfg.file)

	// If print-only requested, stream file to stdout
	if cfg.printLines {
		f, err := os.Open(cfg.file)
		if err != nil {
			log.Fatalf("open file for print: %v", err)
		}
		defer f.Close()
		io.Copy(os.Stdout, f)
		return
	}

	// Verify using streaming reader
	f, err := os.Open(cfg.file)
	if err != nil {
		log.Fatalf("open file: %v", err)
	}
	defer f.Close()

	res, err := merklelog.VerifyReader(f, merklelog.VerifyOptions{HMACKey: []byte(cfg.hmacKey)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: checker I/O: %v\n", err)
		os.Exit(4)
	}
	if !res.OK {
		fmt.Fprintf(os.Stderr, "FAILED: checker: %v (checked %d lines)\n", res.FirstErr, res.Count)
		os.Exit(3)
	}
	fmt.Printf("OK: Log file consistent (checked %d lines)\n", res.Count)
}
