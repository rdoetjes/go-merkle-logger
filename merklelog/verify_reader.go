package merklelog

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// VerifyReader reads newline-delimited JSON entries from r and verifies them incrementally.
func VerifyReader(r io.Reader, opts VerifyOptions) (VerifyResult, error) {
	var prevHash []byte
	var prevSeq uint64
	res := VerifyResult{OK: true}

	s := bufio.NewScanner(r)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := s.Bytes()
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = fmt.Errorf("line %d: invalid json: %v", lineNo, err)
			}
			continue
		}
		res.Count++
		// sequence
		if lineNo == 1 {
			prevSeq = e.Sequence
		} else {
			if e.Sequence != prevSeq+1 {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: sequence %d not equal to prev+1", lineNo, e.Sequence)
				}
			}
			prevSeq = e.Sequence
		}
		// previous hash
		if e.PreviousHash != "" {
			ph, err := hex.DecodeString(e.PreviousHash)
			if err != nil {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: invalid previous_hash hex: %v", lineNo, err)
				}
			} else if prevHash != nil && !hmac.Equal(ph, prevHash) {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: previous_hash mismatch", lineNo)
				}
			}
		}
		// current hash
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload))))
		calc := h.Sum(nil)
		if hex.EncodeToString(calc) != e.CurrentHash {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = fmt.Errorf("line %d: current_hash mismatch", lineNo)
			}
		}
		// signature
		if len(opts.HMACKey) > 0 {
			mac := hmac.New(sha256.New, opts.HMACKey)
			mac.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload), e.CurrentHash)))
			exp := mac.Sum(nil)
			sig, err := base64.StdEncoding.DecodeString(e.Signature)
			if err != nil {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: signature not base64: %v", lineNo, err)
				}
			} else if !hmac.Equal(sig, exp) {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: signature mismatch", lineNo)
				}
			}
		}
		// set prevHash
		if cb, err := hex.DecodeString(e.CurrentHash); err == nil {
			prevHash = cb
		}
	}
	if err := s.Err(); err != nil {
		return res, err
	}
	return res, nil
}
