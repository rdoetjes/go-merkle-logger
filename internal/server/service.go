package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"phonax.com/merkle/proto"
)

// Config holds service configuration
type Config struct {
	Backend string
	LogFile string
}

// Service implements the Logger gRPC service
type Service struct {
	proto.UnimplementedLoggerServer
	cfg      Config
	mu       sync.Mutex // protects chain state and file
	f        *os.File
	seq      int64
	prevHash []byte
	hmacKey  []byte
}

// NewService creates a new logging service. For simplicity we derive HMAC key from env or generate.
func NewService(cfg Config) (*Service, error) {
	if cfg.Backend != "file" && cfg.Backend != "syslog" {
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	s := &Service{cfg: cfg}
	// load or generate HMAC key (in real deployments, this should be loaded from KMS/secret store)
	k := os.Getenv("MERKLE_HMAC_KEY")
	if k == "" {
		// generate a random key (not cryptographically secure here for demo)
		k = "demo-key-please-change"
	}
	s.hmacKey = []byte(k)

	if cfg.Backend == "file" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0700); err != nil {
			return nil, err
		}
		// If a file already exists at the configured log path, rename it to preserve old logs
		// New name: <basename>-<isodatetimestamp><ext> (if ext missing, .log is used)
		if fi, err := os.Stat(cfg.LogFile); err == nil && !fi.IsDir() {
			// file exists, compute new name
			ext := filepath.Ext(cfg.LogFile)
			base := strings.TrimSuffix(cfg.LogFile, ext)
			if ext == "" {
				ext = ".log"
			}
			ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
			newName := fmt.Sprintf("%s-%s%s", base, ts, ext)
			// Attempt to rename; if it fails we'll log and continue (server will append to existing file)
			if err := os.Rename(cfg.LogFile, newName); err != nil {
				log.Printf("warning: failed to rename existing log file %s -> %s: %v", cfg.LogFile, newName, err)
			} else {
				log.Printf("existing log file renamed: %s -> %s", cfg.LogFile, newName)
			}
		} else if err != nil && !os.IsNotExist(err) {
			// unexpected stat error
			log.Printf("warning: could not stat log file %s: %v", cfg.LogFile, err)
		}

		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		s.f = f
		// attempt to restore last sequence and hash from file tail
		if err := s.restoreState(); err != nil {
			// log but continue from zero
			log.Printf("warning: could not restore state: %v", err)
		}
	}

	return s, nil
}

func (s *Service) Close() error {
	if s.f != nil {
		return s.f.Close()
	}
	return nil
}

// Write handles incoming log requests
func (s *Service) Write(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	log.Printf("Write request: app=%q level=%q message=%q fields=%v", req.Application, req.Level, req.Message, req.Fields)

	// Acquire lock for atomic sequence allocation and append
	s.mu.Lock()
	seq := s.seq + 1
	prev := make([]byte, len(s.prevHash))
	copy(prev, s.prevHash)

	// build entry bytes (without updating in-memory state yet)
	entry, curHash, err := s.makeEntryWith(seq, prev, req)
	if err != nil {
		s.mu.Unlock()
		log.Printf("makeEntryWith failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to make entry: %v", err)
	}

	// append to backend while holding lock to ensure order
	if s.cfg.Backend == "file" {
		if err := s.appendToFile(entry); err != nil {
			log.Printf("appendToFile failed: %v", err)
			s.mu.Unlock()
			return &proto.LogResponse{Ok: false, Error: err.Error()}, nil
		}
	} else {
		// syslog path: for demo we just write to stdout with prefix SYSLOG
		fmt.Printf("SYSLOG: %s\n", string(entry))
	}

	// update in-memory chain state only after successful append
	s.seq = seq
	s.prevHash = curHash
	s.mu.Unlock()

	log.Printf("Wrote sequence=%d", seq)
	return &proto.LogResponse{Ok: true}, nil
}

func (s *Service) Verify(ctx context.Context, req *proto.VerifyRequest) (*proto.VerifyResponse, error) {
	// For demo: always return ok
	return &proto.VerifyResponse{Ok: true}, nil
}

// makeEntryWith builds the structured log entry with chaining for given sequence and previous hash
func (s *Service) makeEntryWith(seq int64, prev []byte, req *proto.LogRequest) ([]byte, []byte, error) {
	// build payload
	payload := map[string]interface{}{
		"application": req.Application,
		"level":       req.Level,
		"message":     req.Message,
		"fields":      req.Fields,
	}
	payloadB, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	t := time.Now().UTC().Format(time.RFC3339Nano)

	// compute current hash = H(sequence|timestamp|prev|payload)
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%x|%s", seq, t, prev, payloadB)))
	curHash := h.Sum(nil)

	// compute signature/HMAC over the entry (excluding signature)
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(fmt.Sprintf("%d|%s|%x|%s|%x", seq, t, prev, payloadB, curHash)))
	sig := mac.Sum(nil)

	entryObj := map[string]interface{}{
		"sequence":      seq,
		"timestamp":     t,
		"previous_hash": fmt.Sprintf("%x", prev),
		"payload":       json.RawMessage(payloadB),
		"current_hash":  fmt.Sprintf("%x", curHash),
		"signature":     base64.StdEncoding.EncodeToString(sig),
	}

	entryLine, err := json.Marshal(entryObj)
	if err != nil {
		return nil, nil, err
	}

	// prepare newline terminated
	entryLine = append(entryLine, '\n')
	return entryLine, curHash, nil
}

func (s *Service) appendToFile(entry []byte) error {
	// write and fsync to reduce risk of loss (performance tradeoff)
	if s.f == nil {
		return errors.New("file backend not initialized")
	}

	n, err := s.f.Write(entry)
	if err != nil {
		log.Printf("file write error: %v", err)
		return err
	}
	if n != len(entry) {
		log.Printf("short write: wrote %d expected %d", n, len(entry))
		return fmt.Errorf("short write")
	}
	if err := s.f.Sync(); err != nil {
		log.Printf("fsync error: %v", err)
		return err
	}
	return nil
}

// restoreState loads last entry to set s.seq and s.prevHash
func (s *Service) restoreState() error {
	if s.f == nil {
		return errors.New("file not initialized")
	}

	// attempt to read last 32KB
	stat, err := s.f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	var start int64 = 0
	if size > 32768 {
		start = size - 32768
	}
	buf := make([]byte, size-start)
	if _, err := s.f.ReadAt(buf, start); err != nil && err != io.EOF {
		return err
	}

	// find last newline
	idx := int64(-1)
	for i := int64(len(buf)) - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("no complete log entry found")
	}
	last := buf[idx+1:]
	var entry map[string]interface{}
	if err := json.Unmarshal(last, &entry); err != nil {
		return err
	}
	if seqv, ok := entry["sequence"].(float64); ok {
		s.seq = int64(seqv)
	}
	if ph, ok := entry["current_hash"].(string); ok {
		b, err := hexDecodeString(ph)
		if err == nil {
			s.prevHash = b
		}
	}
	return nil
}

func hexDecodeString(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
