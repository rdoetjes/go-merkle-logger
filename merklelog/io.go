package merklelog

import (
	"errors"
	"os"
)

// AppendToFile writes an entry for the given parameters to file f and fsyncs it.
// Returns the current hash (raw bytes) and the written entry bytes.
func AppendToFile(f *os.File, seq uint64, prev []byte, payload interface{}, hmacKey []byte) ([]byte, []byte, error) {
	if f == nil {
		return nil, nil, errors.New("file is nil")
	}
	entry, curHash, err := MakeEntry(seq, prev, payload, hmacKey)
	if err != nil {
		return nil, nil, err
	}
	if _, err := f.Write(entry); err != nil {
		return nil, nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, nil, err
	}
	return curHash, entry, nil
}
