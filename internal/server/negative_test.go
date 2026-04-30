package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewServiceInvalidBackend(t *testing.T) {
	_, err := NewService(Config{Backend: "invalid", LogFile: "x"})
	require.Error(t, err)
}

func TestLoadHMACKeyFromEnv(t *testing.T) {
	os.Setenv("MERKLE_HMAC_KEY", "envkey")
	k := loadHMACKey()
	require.Equal(t, []byte("envkey"), k)
}
