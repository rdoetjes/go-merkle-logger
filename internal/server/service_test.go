package server

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"phonax.com/merkle/proto"
)

func TestWriteAndRestoreState(t *testing.T) {
	// prepare temp dir
	d := t.TempDir()
	logdir := filepath.Join(d, "logs")
	_ = os.MkdirAll(logdir, 0o700)
	logfile := filepath.Join(logdir, "protected.log")

	// set deterministic hmac key
	os.Setenv("MERKLE_HMAC_KEY", "test-key-1234")

	svc, err := NewService(Config{Backend: "file", LogFile: logfile})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer svc.Close()

	// write one entry
	ctx := context.Background()
	res, err := svc.Write(ctx, &proto.LogRequest{Application: "app1", Level: "info", Message: "first"})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if !res.Ok {
		t.Fatalf("Write response not ok: %v", res.Error)
	}

	// check file contains one JSON line
	f, err := os.Open(logfile)
	if err != nil {
		t.Fatalf("open logfile: %v", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("expected at least one line in logfile")
	}
	line := sc.Text()
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("invalid json in logfile: %v", err)
	}
	// sequence should be 1
	if seq, ok := entry["sequence"].(float64); !ok || int64(seq) != 1 {
		t.Fatalf("expected sequence 1, got %v", entry["sequence"])
	}

	// verify current_hash matches recomputed hash
	curHashStr, _ := entry["current_hash"].(string)
	prevHashStr, _ := entry["previous_hash"].(string)
	payloadRaw, _ := entry["payload"].(map[string]interface{})
	payloadB, _ := json.Marshal(payloadRaw)
	h := sha256.New()
	h.Write([]byte(strings.Join([]string{"1", entry["timestamp"].(string), prevHashStr, string(payloadB)}, "|")))
	calc := h.Sum(nil)
	if hex.EncodeToString(calc) != curHashStr {
		t.Fatalf("current_hash mismatch: got %s expected %s", curHashStr, hex.EncodeToString(calc))
	}

	// verify signature using HMAC key
	sigB, err := base64.StdEncoding.DecodeString(entry["signature"].(string))
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	mac := hmac.New(sha256.New, []byte("test-key-1234"))
	mac.Write([]byte(strings.Join([]string{"1", entry["timestamp"].(string), prevHashStr, string(payloadB), curHashStr}, "|")))
	exp := mac.Sum(nil)
	if !hmac.Equal(sigB, exp) {
		t.Fatalf("signature verification failed")
	}

	// close service then reopen to test restoreState
	svc.Close()
	svc2, err := NewService(Config{Backend: "file", LogFile: logfile})
	if err != nil {
		t.Fatalf("NewService restore failed: %v", err)
	}
	defer svc2.Close()

	// write another entry and ensure sequence increments to 2
	res2, err := svc2.Write(ctx, &proto.LogRequest{Application: "app1", Level: "info", Message: "second"})
	if err != nil || !res2.Ok {
		t.Fatalf("second write failed: %v %v", err, res2)
	}

	// read last line and check sequence 2
	func() {
		f2, err := os.Open(logfile)
		if err != nil {
			t.Fatalf("open logfile: %v", err)
		}
		defer f2.Close()
		var last string
		sc2 := bufio.NewScanner(f2)
		for sc2.Scan() {
			last = sc2.Text()
		}
		if last == "" {
			t.Fatalf("expected lines in logfile")
		}
		var e2 map[string]interface{}
		if err := json.Unmarshal([]byte(last), &e2); err != nil {
			t.Fatalf("invalid json last line: %v", err)
		}
		if seqF, ok := e2["sequence"].(float64); !ok {
			t.Fatalf("expected numeric sequence, got %v", e2["sequence"])
		} else {
			seq := int64(seqF)
			if seq == 2 {
				// ok: chain continued
			} else if seq == 1 {
				// rotation happened on startup; ensure rotated file contains the prior entry
				ext := filepath.Ext(logfile)
				base := strings.TrimSuffix(logfile, ext)
				pattern := base + "-*" + ext
				matches, err := filepath.Glob(pattern)
				if err != nil {
					t.Fatalf("glob failed: %v", err)
				}
				if len(matches) == 0 {
					t.Fatalf("expected rotated file after startup but none found")
				}
				found := false
				for _, m := range matches {
					b, err := os.ReadFile(m)
					if err != nil {
						continue
					}
					var ent map[string]interface{}
					if err := json.Unmarshal(b, &ent); err != nil {
						continue
					}
					if sv, ok := ent["sequence"].(float64); ok && int64(sv) == 1 {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("rotated files did not contain prior sequence 1 entry")
				}
			} else {
				t.Fatalf("unexpected sequence %d", seq)
			}
		}
	}()
}

func TestRotateExistingFileOnStartup(t *testing.T) {
	// prepare temp dir and create an existing logfile
	d := t.TempDir()
	logdir := filepath.Join(d, "x")
	_ = os.MkdirAll(logdir, 0o700)
	logfile := filepath.Join(logdir, "app.log")
	if err := os.WriteFile(logfile, []byte("old content\n"), 0o600); err != nil {
		t.Fatalf("create existing file: %v", err)
	}

	os.Setenv("MERKLE_HMAC_KEY", "test-key-1234")

	svc, err := NewService(Config{Backend: "file", LogFile: logfile})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer svc.Close()

	// original file should have been renamed; new logfile exists
	if _, err := os.Stat(logfile); err != nil {
		t.Fatalf("expected new logfile to exist, stat error: %v", err)
	}
	// search for rotated file with prefix base-<ts>
	base := strings.TrimSuffix(logfile, filepath.Ext(logfile))
	pattern := base + "-*" + filepath.Ext(logfile)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected rotated file matching %s, found none", pattern)
	}
	// ensure one of the matches contains the original content
	foundOld := false
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "old content") {
			foundOld = true
			break
		}
	}
	if !foundOld {
		t.Fatalf("rotated file did not contain original content")
	}
}

func TestCheckerCompatibility(t *testing.T) {
	// ensure checker would accept the file format produced by service
	d := t.TempDir()
	logdir := filepath.Join(d, "logs")
	_ = os.MkdirAll(logdir, 0o700)
	logfile := filepath.Join(logdir, "protected.log")
	os.Setenv("MERKLE_HMAC_KEY", "test-key-1234")

	svc, err := NewService(Config{Backend: "file", LogFile: logfile})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer svc.Close()
	ctx := context.Background()
	if _, err := svc.Write(ctx, &proto.LogRequest{Application: "a", Level: "i", Message: "m"}); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// run checker logic inline: open file, read, parse and verify signature
	f, err := os.Open(logfile)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatalf("no line found")
	}
	var e struct {
		Sequence     int64           `json:"sequence"`
		Timestamp    string          `json:"timestamp"`
		PreviousHash string          `json:"previous_hash"`
		Payload      json.RawMessage `json:"payload"`
		CurrentHash  string          `json:"current_hash"`
		Signature    string          `json:"signature"`
	}
	if err := json.Unmarshal([]byte(sc.Text()), &e); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	// recompute and verify signature
	h := sha256.New()
	h.Write([]byte(strings.Join([]string{"1", e.Timestamp, e.PreviousHash, string(e.Payload)}, "|")))
	calc := h.Sum(nil)
	if hex.EncodeToString(calc) != e.CurrentHash {
		t.Fatalf("current_hash mismatch")
	}
	mac := hmac.New(sha256.New, []byte("test-key-1234"))
	mac.Write([]byte(strings.Join([]string{"1", e.Timestamp, e.PreviousHash, string(e.Payload), e.CurrentHash}, "|")))
	exp := mac.Sum(nil)
	sig, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil {
		t.Fatalf("signature decode: %v", err)
	}
	if !hmac.Equal(sig, exp) {
		t.Fatalf("signature verify failed")
	}
	// scanner err check
	if err := sc.Err(); err != nil && err != io.EOF {
		t.Fatalf("scanner error: %v", err)
	}
}
