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

// helper: parse a line into Entry
func parseLine(line []byte) (Entry, error) {
	var e Entry
	if err := json.Unmarshal(line, &e); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// helper: check sequence monotonicity
func checkSequence(i int, e Entry, prevSeq *uint64) error {
	if i == 0 {
		*prevSeq = e.Sequence
		return nil
	}
	if e.Sequence != *prevSeq+1 {
		return fmt.Errorf("line %d: sequence %d not equal to prev+1", i+1, e.Sequence)
	}
	*prevSeq = e.Sequence
	return nil
}

// helper: verify previous hash matches
func checkPrevHash(i int, e Entry, prevHash []byte) error {
	if e.PreviousHash == "" {
		return nil
	}
	ph, err := hex.DecodeString(e.PreviousHash)
	if err != nil {
		return fmt.Errorf("line %d: invalid previous_hash hex: %v", i+1, err)
	}
	if prevHash != nil && !hmac.Equal(ph, prevHash) {
		return fmt.Errorf("line %d: previous_hash mismatch", i+1)
	}
	return nil
}

// helper: compute current hash and compare
func computeAndCheckCurrentHash(i int, e Entry) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload))))
	calc := h.Sum(nil)
	if hex.EncodeToString(calc) != e.CurrentHash {
		return calc, fmt.Errorf("line %d: current_hash mismatch", i+1)
	}
	return calc, nil
}

// helper: check signature if key provided
func checkSignature(i int, e Entry, opts VerifyOptions) error {
	if len(opts.HMACKey) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, opts.HMACKey)
	mac.Write([]byte(fmt.Sprintf("%d|%s|%s|%s|%s", e.Sequence, e.Timestamp, e.PreviousHash, string(e.Payload), e.CurrentHash)))
	exp := mac.Sum(nil)
	sig, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil {
		return fmt.Errorf("line %d: signature not base64: %v", i+1, err)
	}
	if !hmac.Equal(sig, exp) {
		return fmt.Errorf("line %d: signature mismatch", i+1)
	}
	return nil
}

// verifyEntry performs all checks for a single parsed Entry and returns updated prevHash and prevSeq.
func verifyEntry(i int, e Entry, prevHash []byte, prevSeq uint64, opts VerifyOptions) ([]byte, uint64, error) {
	if err := checkSequence(i, e, &prevSeq); err != nil {
		return prevHash, prevSeq, err
	}
	if err := checkPrevHash(i, e, prevHash); err != nil {
		return prevHash, prevSeq, err
	}
	calc, err := computeAndCheckCurrentHash(i, e)
	if err != nil {
		return prevHash, prevSeq, err
	}
	if err := checkSignature(i, e, opts); err != nil {
		return prevHash, prevSeq, err
	}
	if cb, err := hex.DecodeString(e.CurrentHash); err == nil {
		prevHash = cb
	}
	_ = calc // calc is the raw bytes of current hash (not currently used by caller)
	return prevHash, prevSeq, nil
}

// VerifyStream verifies a sequence of newline-terminated JSON entries provided as lines.
// It verifies monotonic sequence numbers, hash chaining, current_hash correctness and (optional) signatures.
func VerifyStream(lines [][]byte, opts VerifyOptions) (VerifyResult, error) {
	var prevHash []byte
	var prevSeq uint64 = 0
	res := VerifyResult{OK: true}
	for i, line := range lines {
		e, err := parseLine(line)
		if err != nil {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = fmt.Errorf("line %d: invalid json: %v", i+1, err)
			}
			continue
		}
		res.Count++
		if _, _, err := verifyEntry(i, e, prevHash, prevSeq, opts); err != nil {
			res.OK = false
			if res.FirstErr == nil {
				res.FirstErr = err
			}
			// if verifyEntry failed, we might still want to update prevHash/prevSeq? keep previous state
			continue
		}
		// update prevHash/prevSeq based on parsed entry
		if cb, err := hex.DecodeString(e.CurrentHash); err == nil {
			prevHash = cb
		}
		prevSeq = e.Sequence
	}
	return res, nil
}
