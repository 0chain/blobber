package reference

import (
	"0chain.net/blobbercore/config"
	"github.com/stretchr/testify/require"
	"fmt"
	"testing"
)

func TestSecretKeys(t *testing.T) {
	config.Configuration = config.Config{
		SignatureScheme: "ed25519",
	}
	result, err := GetSecretKeyPair()

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.PublicKey)
	require.NotNil(t, result.PrivateKey)
	fmt.Println(result.PublicKey, result.PrivateKey)
}
