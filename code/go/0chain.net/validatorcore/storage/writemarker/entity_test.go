package writemarker_test

import (
	"fmt"
	"testing"

	"0chain.net/core/common"
	"0chain.net/core/config"
	"0chain.net/core/encryption"
	"0chain.net/validatorcore/storage/writemarker"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMarker_GetHashData(t *testing.T) {
	wm, wallet, err := setupEntityTest(t)
	require.NoError(t, err)

	want := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", "alloc_root", "prev_alloc_root", "alloc_id", "blobber_id", wallet.ClientID, 1, common.Now())
	got := wm.GetHashData()
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestWriteMarker_VerifySignature(t *testing.T) {
	wm, wallet, err := setupEntityTest(t)
	require.NoError(t, err)

	tests := []struct {
		name      string
		publicKey string
		want      bool
	}{
		{
			name:      "valid",
			publicKey: wallet.Keys[0].PublicKey,
			want:      true,
		},
		{
			name:      "invalid",
			publicKey: "1",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wm.VerifySignature(tt.publicKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteMarker_Verify(t *testing.T) {
	wm, wallet, err := setupEntityTest(t)
	require.NoError(t, err)

	tests := []struct {
		name      string
		allocID   string
		allocRoot string
		publicKey string
		wantErr   bool
	}{
		{
			name:      "valid",
			allocID:   "alloc_id",
			allocRoot: "alloc_root",
			publicKey: wallet.Keys[0].PublicKey,
			wantErr:   false,
		},
		{
			name:      "invalid: wrong Allocation ID",
			allocID:   "id",
			allocRoot: "alloc_root",
			publicKey: wallet.Keys[0].PublicKey,
			wantErr:   true,
		},
		{
			name:      "invalid: wrong Allocation Root",
			allocID:   "alloc_id",
			allocRoot: "root",
			publicKey: wallet.Keys[0].PublicKey,
			wantErr:   true,
		},
		{
			name:      "invalid: wrong Public Key",
			allocID:   "alloc_id",
			allocRoot: "alloc_root",
			publicKey: "1",
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := wm.Verify(tt.allocID, tt.allocRoot, tt.publicKey)
			if !tt.wantErr {
				require.NoError(t, err)
			}
		})
	}
}

func setupEntityTest(t *testing.T) (*writemarker.WriteMarker, *zcncrypto.Wallet, error) {
	t.Helper()
	config.Configuration = config.Config{
		SignatureScheme: "bls0chain",
	}
	wm := &writemarker.WriteMarker{
		AllocationRoot:         "alloc_root",
		PreviousAllocationRoot: "prev_alloc_root",
		AllocationID:           "alloc_id",
		Size:                   int64(1),
		BlobberID:              "blobber_id",
		Timestamp:              common.Now(),
	}

	sigSch := zcncrypto.NewSignatureScheme("bls0chain")
	wallet, err := sigSch.GenerateKeys()
	if err != nil {
		return wm, wallet, err
	}
	wm.ClientID = wallet.ClientID

	sigSch.SetPrivateKey(wallet.Keys[0].PrivateKey)
	sig, err := sigSch.Sign(encryption.Hash(wm.GetHashData()))
	if err != nil {
		return wm, wallet, err
	}
	wm.Signature = sig
	return wm, wallet, nil
}
