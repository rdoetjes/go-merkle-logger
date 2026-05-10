package merklelog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Entry represents the structured log entry format persisted to disk
type Entry struct {
	Sequence     uint64          `json:"sequence"`
	Timestamp    string          `json:"timestamp"`
	PreviousHash string          `json:"previous_hash"`
	Payload      json.RawMessage `json:"payload"`
	CurrentHash  string          `json:"current_hash"`
	Signature    string          `json:"signature"`
}

// MakeEntry builds a JSON newline-terminated entry and returns the bytes and the current hash.
// - seq: monotonic sequence number
// - prev: previous hash bytes (may be nil)
// - payloadObj: any JSON-marshalable payload
// - hmacKey: raw HMAC key, if empty signature will be empty
func MakeEntry(seq uint64, prev []byte, payloadObj interface{}, hmacKey []byte) ([]byte, []byte, error) {
	payloadB, err := json.Marshal(payloadObj)
	if err != nil {
		return nil, nil, err
	}
	t := time.Now().UTC().Format(time.RFC3339Nano)

	// compute current hash = H(sequence|timestamp|prev|payload)
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%x|%s", seq, t, prev, payloadB)))
	curHash := h.Sum(nil)

	// compute signature/HMAC over the entry (excluding signature)
	var sigB []byte
	if len(hmacKey) > 0 {
		mac := hmac.New(sha256.New, hmacKey)
		mac.Write([]byte(fmt.Sprintf("%d|%s|%x|%s|%x", seq, t, prev, payloadB, curHash)))
		sigB = mac.Sum(nil)
	}

	entry := Entry{
		Sequence:     seq,
		Timestamp:    t,
		PreviousHash: fmt.Sprintf("%x", prev),
		Payload:      json.RawMessage(payloadB),
		CurrentHash:  fmt.Sprintf("%x", curHash),
		Signature:    base64.StdEncoding.EncodeToString(sigB),
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return nil, nil, err
	}
	b = append(b, '\n')
	return b, curHash, nil
}

// Helper to verify entry bytes (returns parsed Entry and computed current hash)
func ParseAndVerifyEntry(line []byte) (Entry, []byte, error) {
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return Entry{}, nil, err
	}
	// recompute hash
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload))))
	calc := h.Sum(nil)
	return e, calc, nil
}
