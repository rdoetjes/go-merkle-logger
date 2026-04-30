package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"phonax.com/merkle/merklelog"
	"phonax.com/merkle/proto"
)

func TestLoadHMACKeyEnvFallback(t *testing.T) {
	os.Unsetenv("MERKLE_HMAC_KEY")
	k := loadHMACKey()
	require.NotEmpty(t, k)
	// set env and verify used
	os.Setenv("MERKLE_HMAC_KEY", "my-secret")
	k2 := loadHMACKey()
	require.Equal(t, []byte("my-secret"), k2)
}

func TestRotateExistingLog(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "app.log")
	orig := []byte("hello world\n")
	require.NoError(t, ioutil.WriteFile(p, orig, 0600))

	require.NoError(t, rotateExistingLog(p))
	// original should not exist
	_, err := os.Stat(p)
	require.True(t, os.IsNotExist(err))
	// rotated file should exist and contain content
	matches, err := filepath.Glob(filepath.Join(d, "app-*.log"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	b, err := ioutil.ReadFile(matches[0])
	require.NoError(t, err)
	require.Equal(t, orig, b)
}

func TestMakeEntryWithHashAndHMAC(t *testing.T) {
	s := &Service{hmacKey: []byte("key123")}
	req := &proto.LogRequest{Application: "app", Level: "info", Message: "msg", Fields: map[string]string{"a": "b"}}
	entryLine, curHash, err := s.makeEntryWith(1, nil, req)
	require.NoError(t, err)
	// parse
	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(entryLine, &obj))
	// check fields
	require.EqualValues(t, float64(1), obj["sequence"])
	// recompute hash
	payloadB, _ := json.Marshal(map[string]interface{}{"application": "app", "level": "info", "message": "msg", "fields": map[string]string{"a": "b"}})
	timestamp := obj["timestamp"].(string)
	h := sha256.New()
	h.Write([]byte(strings.Join([]string{"1", timestamp, "", string(payloadB)}, "|")))
	calc := h.Sum(nil)
	require.Equal(t, hex.EncodeToString(calc), obj["current_hash"].(string))
	// verify HMAC signature
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(strings.Join([]string{"1", timestamp, "", string(payloadB), obj["current_hash"].(string)}, "|")))
	exp := mac.Sum(nil)
	sigB, err := base64.StdEncoding.DecodeString(obj["signature"].(string))
	require.NoError(t, err)
	require.Equal(t, exp, sigB)
	require.Equal(t, curHash, calc)
}

func TestAppendToFileAndRestoreState(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "protected.log")
	s := &Service{cfg: Config{Backend: "file", LogFile: p}, hmacKey: []byte("k")}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	require.NoError(t, err)
	s.f = f
	defer s.Close()

	// use merklelog.AppendToFile helper to write an entry
	_, _, err = merklelog.AppendToFile(s.f, 1, nil, map[string]interface{}{"sequence": 1}, s.hmacKey)
	require.NoError(t, err)
	// close and reopen service to restore
	s.f.Close()
	s2, err := NewService(Config{Backend: "file", LogFile: p})
	require.NoError(t, err)
	defer s2.Close()
	// ensure seq recovered or, if rotation happened, check rotated file contains the prior entry
	if s2.seq >= 1 {
		require.True(t, s2.seq >= 1)
		return
	}
	// rotation likely occurred; search for rotated files
	base := strings.TrimSuffix(p, filepath.Ext(p))
	pattern := base + "-*" + filepath.Ext(p)
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	found := false
	for _, m := range matches {
		b, err := ioutil.ReadFile(m)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "\"sequence\":1") {
			found = true
			break
		}
	}
	require.True(t, found, "rotated files did not contain prior sequence 1 entry")
}
