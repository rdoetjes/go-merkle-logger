package merklelog

import (
	"os"
)

// VerifyFile opens the file at path and verifies it using VerifyReader.
// It returns the VerifyResult and any I/O error encountered while opening/reading the file.
func VerifyFile(path string, opts VerifyOptions) (VerifyResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return VerifyResult{OK: false}, err
	}
	defer f.Close()
	return VerifyReader(f, opts)
}
