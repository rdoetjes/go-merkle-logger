package merklelog

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// VerifyOptions control verification behavior
type VerifyOptions struct {
	HMACKey []byte // if non-empty, verify signatures
}

// VerifyResult contains verification results
type VerifyResult struct {
	OK       bool
	FirstErr error
	Count    int
}

// VerifyStream verifies a sequence of newline-terminated JSON entries provided as lines.
// It verifies monotonic sequence numbers, hash chaining, current_hash correctness and (optional) signatures.
func VerifyStream(lines [][]byte, opts VerifyOptions) (VerifyResult, error) {
	var prevHash []byte
	var prevSeq uint64 = 0
	res := VerifyResult{OK: true}
	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = fmt.Errorf("line %d: invalid json: %v", i+1, err)
			}
			continue
		}
		res.Count++
		// sequence monotonic check
		if i == 0 {
			// accept any start sequence
			prevSeq = e.Sequence
		} else {
			if e.Sequence != prevSeq+1 {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: sequence %d not equal to prev+1", i+1, e.Sequence)
				}
			}
			prevSeq = e.Sequence
		}
		// verify previous hash matches
		if e.PreviousHash != "" {
			ph, err := hex.DecodeString(e.PreviousHash)
			if err != nil {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: invalid previous_hash hex: %v", i+1, err)
				}
			} else if prevHash != nil && !hmac.Equal(ph, prevHash) {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: previous_hash mismatch", i+1)
				}
			}
		}
		// recompute current hash
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload))))
		calc := h.Sum(nil)
		if hex.EncodeToString(calc) != e.CurrentHash {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = fmt.Errorf("line %d: current_hash mismatch", i+1)
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
					res.FirstErr = fmt.Errorf("line %d: signature not base64: %v", i+1, err)
				}
			} else if !hmac.Equal(sig, exp) {
				res.OK = false
				if res.FirstErr == nil {
					res.FirstErr = fmt.Errorf("line %d: signature mismatch", i+1)
				}
			}
		}
		// set prevHash
		if cb, err := hex.DecodeString(e.CurrentHash); err == nil {
			prevHash = cb
		}
	}
	return res, nil
}
