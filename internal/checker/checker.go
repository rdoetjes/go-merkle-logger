package checker

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// CheckFile verifies the log file at path using the provided HMAC key (raw or base64).
// Returns nil when file is consistent; otherwise returns an error with details.
func CheckFile(path string, hmacKey string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	var prevHash []byte
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := s.Bytes()
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			return fmt.Errorf("line %d: invalid json: %w", lineNo, err)
		}
		// recompute current hash
		seq := int64(entry["sequence"].(float64))
		ts := entry["timestamp"].(string)
		prev := entry["previous_hash"].(string)
		payloadB, _ := json.Marshal(entry["payload"])
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", seq, ts, prev, string(payloadB))))
		calc := h.Sum(nil)
		if hex.EncodeToString(calc) != entry["current_hash"].(string) {
			return fmt.Errorf("line %d: current hash mismatch", lineNo)
		}
		// verify previous
		if prev != "" {
			ph, err := hex.DecodeString(prev)
			if err != nil {
				return fmt.Errorf("line %d: previous_hash not hex", lineNo)
			}
			if prevHash != nil && !hmac.Equal(ph, prevHash) {
				return fmt.Errorf("line %d: previous hash mismatch", lineNo)
			}
		}
		// verify signature if provided
		if hmacKey != "" {
			var key []byte
			if kb, err := base64.StdEncoding.DecodeString(hmacKey); err == nil {
				key = kb
			} else {
				key = []byte(hmacKey)
			}
			mac := hmac.New(sha256.New, key)
			mac.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s", seq, ts, prev, string(payloadB), entry["current_hash"].(string))))
			exp := mac.Sum(nil)
			sigB, err := base64.StdEncoding.DecodeString(entry["signature"].(string))
			if err != nil {
				return fmt.Errorf("line %d: signature not base64", lineNo)
			}
			if !hmac.Equal(sigB, exp) {
				return fmt.Errorf("line %d: signature mismatch", lineNo)
			}
		}
		// set prev
		if cb, err := hex.DecodeString(entry["current_hash"].(string)); err == nil {
			prevHash = cb
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}
