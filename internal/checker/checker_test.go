package checker

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFileSuccess(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "log.log")
	// construct payload consistently
	payload := map[string]interface{}{"application": "a"}
	payloadB, _ := json.Marshal(payload)
	ts := "2026-01-01T00:00:00Z"
	// compute current hash according to CheckFile semantics: H(sequence|timestamp|previous|payload)
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d|%s|%s|%s", 1, ts, "", string(payloadB))))
	calc := h.Sum(nil)
	curHex := hex.EncodeToString(calc)
	// build entry JSON
	entryObj := map[string]interface{}{
		"sequence":      1,
		"timestamp":     ts,
		"previous_hash": "",
		"payload":       payload,
		"current_hash":  curHex,
		"signature":     "",
	}
	entryB, _ := json.Marshal(entryObj)
	ioutil.WriteFile(p, entryB, 0600)
	ioutil.WriteFile(p, append(entryB, '\n'), 0600)
	// signature empty and hmac key empty -> should succeed
	if err := CheckFile(p, ""); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestCheckFileSignature(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "log.log")
	// build a minimal entry with signature using key
	key := []byte("mykey")
	payload := `{"application":"a"}`
	// precomputed hash of empty previous and payload with seq=1 and ts
	hash := "dummy"
	// we will compute a signature and place it
	sig := base64.StdEncoding.EncodeToString([]byte("sig"))
	entry := `{"sequence":1,"timestamp":"2026-01-01T00:00:00Z","previous_hash":"","payload":` + payload + `,"current_hash":"` + hash + `","signature":"` + sig + `"}` + "\n"
	ioutil.WriteFile(p, []byte(entry), 0600)
	// now check should fail because hash/signature won't match actual
	if err := CheckFile(p, string(key)); err == nil {
		t.Fatalf("expected failure due to bad signature")
	}
	// cleanup
	os.Remove(p)
}
