package server

import (
	"bytes"
	"context"
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
	"phonax.com/merkle/merklelog"
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
	seq      uint64
	prevHash []byte
	hmacKey  []byte
}

// NewService creates a new logging service. For simplicity we derive HMAC key from env or generate.
func NewService(cfg Config) (*Service, error) {
	if cfg.Backend != "file" && cfg.Backend != "syslog" {
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	s := &Service{cfg: cfg}

	// load HMAC key
	s.hmacKey = loadHMACKey()

	// initialize backend-specific resources
	if cfg.Backend == "file" {
		if err := s.initFileBackend(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// loadHMACKey reads HMAC key from env or returns a demo key
func loadHMACKey() []byte {
	k := os.Getenv("MERKLE_HMAC_KEY")
	if k == "" {
		// generate a random key (not cryptographically secure here for demo)
		k = "demo-key-please-change"
	}
	return []byte(k)
}

// initFileBackend prepares the directory, rotates any existing logfile, opens the new file and restores state
func (s *Service) initFileBackend() error {
	if err := os.MkdirAll(filepath.Dir(s.cfg.LogFile), 0700); err != nil {
		return err
	}

	// rotate existing file if present
	if err := rotateExistingLog(s.cfg.LogFile); err != nil {
		// rotation errors are non-fatal; they are logged inside rotateExistingLog
		// continue to open the file below
	}

	f, err := os.OpenFile(s.cfg.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	s.f = f

	// attempt to restore last sequence and hash from file tail
	if err := s.restoreState(); err != nil {
		// log but continue from zero
		log.Printf("warning: could not restore state: %v", err)
	}
	return nil
}

// rotateExistingLog renames an existing logfile to a timestamped name. It logs warnings on failure but does not return fatal errors.
func rotateExistingLog(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Printf("warning: could not stat log file %s: %v", path, err)
		return err
	}

	if fi.IsDir() {
		return nil
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	if ext == "" {
		ext = ".log"
	}

	ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	newName := fmt.Sprintf("%s-%s%s", base, ts, ext)
	if err := os.Rename(path, newName); err != nil {
		log.Printf("warning: failed to rename existing log file %s -> %s: %v", path, newName, err)
		return err
	}
	log.Printf("existing log file renamed: %s -> %s", path, newName)
	return nil
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

	// append to backend while holding lock to ensure order
	if s.cfg.Backend == "file" {
		curHash, _, err := merklelog.AppendToFile(s.f, seq, prev, map[string]interface{}{"application": req.Application, "level": req.Level, "message": req.Message, "fields": req.Fields}, s.hmacKey)
		if err != nil {
			log.Printf("appendToFile failed: %v", err)
			s.mu.Unlock()
			return &proto.LogResponse{Ok: false, Error: err.Error()}, nil
		}
		// update in-memory chain state only after successful append
		s.seq = seq
		s.prevHash = curHash
		s.mu.Unlock()
	} else {
		// syslog path: for demo we just write to stdout with prefix SYSLOG
		entry, curHash, err := merklelog.MakeEntry(seq, prev, map[string]interface{}{"application": req.Application, "level": req.Level, "message": req.Message, "fields": req.Fields}, s.hmacKey)
		if err != nil {
			s.mu.Unlock()
			log.Printf("makeEntry failed for syslog: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to make entry: %v", err)
		}
		fmt.Printf("SYSLOG: %s\n", string(entry))
		s.seq = seq
		s.prevHash = curHash
		s.mu.Unlock()
	}

	log.Printf("Wrote sequence=%d", seq)
	return &proto.LogResponse{Ok: true}, nil
}

func (s *Service) Verify(ctx context.Context, req *proto.VerifyRequest) (*proto.VerifyResponse, error) {
	// For demo: always return ok
	return &proto.VerifyResponse{Ok: true}, nil
}

// makeEntryWith builds the structured log entry with chaining for given sequence and previous hash
// This now delegates the actual entry construction to the shared merklelog package.
func (s *Service) makeEntryWith(seq uint64, prev []byte, req *proto.LogRequest) ([]byte, []byte, error) {
	payload := map[string]interface{}{
		"application": req.Application,
		"level":       req.Level,
		"message":     req.Message,
		"fields":      req.Fields,
	}
	entryLine, curHash, err := merklelog.MakeEntry(seq, prev, payload, s.hmacKey)
	return entryLine, curHash, err
}

// restoreState loads last entry to set s.seq and s.prevHash
// This function is tolerant: if the file is empty or the tail doesn't contain a complete
// JSON entry (e.g., newly rotated file or truncated write), it will treat the state as
// "no previous entries" instead of returning an error.
func (s *Service) restoreState() error {
	if s.f == nil {
		return errors.New("file not initialized")
	}

	// attempt to read last 32KB (or whole file if smaller)
	stat, err := s.f.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()
	if size == 0 {
		// nothing to restore
		return nil
	}
	var start int64 = 0
	if size > 32768 {
		start = size - 32768
	}
	buf := make([]byte, size-start)
	if _, err := s.f.ReadAt(buf, start); err != nil && err != io.EOF {
		return err
	}

	// find last newline; if none, try to parse entire buffer as a single entry
	idx := bytes.LastIndexByte(buf, '\n')
	var last []byte
	if idx == -1 {
		// no newline: the buffer might contain a single truncated or single-line entry
		trim := bytes.TrimSpace(buf)
		if len(trim) == 0 {
			// nothing to restore
			return nil
		}
		last = trim
	} else {
		last = buf[idx+1:]
		if len(bytes.TrimSpace(last)) == 0 {
			// trailing newline but nothing after it; try to find previous non-empty line
			buf2 := bytes.TrimRight(buf[:idx], "\n")
			idx2 := bytes.LastIndexByte(buf2, '\n')
			if idx2 == -1 {
				last = bytes.TrimSpace(buf2)
			} else {
				last = bytes.TrimSpace(buf2[idx2+1:])
			}
		}
	}

	if len(last) == 0 {
		// no complete entry found; nothing to restore
		return nil
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(last, &entry); err != nil {
		// malformed last entry: treat as "no restore" rather than fatal
		log.Printf("warning: could not parse last log entry during restore: %v", err)
		return nil
	}
	if seqv, ok := entry["sequence"].(float64); ok {
		s.seq = uint64(seqv)
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
