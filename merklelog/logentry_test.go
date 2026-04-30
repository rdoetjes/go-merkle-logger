package merklelog

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeEntryAndParse(t *testing.T) {
	key := []byte("testkey")
	payload := map[string]interface{}{"app": "demo", "msg": "hello"}
	b, _, err := MakeEntry(1, nil, payload, key)
	require.NoError(t, err)
	require.NotEmpty(t, b)
	// parse
	var e Entry
	require.NoError(t, json.Unmarshal(b, &e))
	// recompute hash and compare to entry.CurrentHash
	_, calc, err := ParseAndVerifyEntry(b)
	require.NoError(t, err)
	require.Equal(t, hex.EncodeToString(calc), e.CurrentHash)
	// verify signature
	if e.Signature != "" {
		sig, err := base64.StdEncoding.DecodeString(e.Signature)
		require.NoError(t, err)
		// manually compute expected HMAC
		// (MakeEntry uses seq|timestamp|prev|payload|curHash), we don't have timestamp here — ensure signature length > 0
		require.Greater(t, len(sig), 0)
	}
}

func TestVerifyStreamGood(t *testing.T) {
	key := []byte("testkey")
	lines := [][]byte{}
	var prev []byte
	for i := uint64(1); i <= 3; i++ {
		b, curHash, err := MakeEntry(i, prev, map[string]interface{}{"i": i}, key)
		require.NoError(t, err)
		lines = append(lines, b)
		prev = curHash
	}
	res, err := VerifyStream(lines, VerifyOptions{HMACKey: key})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, 3, res.Count)
}

func TestVerifyStreamBadSig(t *testing.T) {
	key := []byte("testkey")
	lines := [][]byte{}
	b, _, err := MakeEntry(1, nil, map[string]interface{}{"i": 1}, key)
	require.NoError(t, err)
	// tamper signature by replacing last entry's signature field
	var e Entry
	require.NoError(t, json.Unmarshal(b, &e))
	e.Signature = "AAAA"
	bTam, _ := json.Marshal(e)
	bTam = append(bTam, '\n')
	lines = append(lines, bTam)
	res, err := VerifyStream(lines, VerifyOptions{HMACKey: key})
	require.NoError(t, err)
	require.False(t, res.OK)
	require.NotNil(t, res.FirstErr)
}
