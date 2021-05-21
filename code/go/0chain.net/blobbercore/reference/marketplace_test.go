package reference

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSecretKeys(t *testing.T) {
	result, err := GetSecretKeyPair()

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.PublicKey)
	require.NotNil(t, result.PrivateKey)
	fmt.Println(result.PublicKey, result.PrivateKey)
}
