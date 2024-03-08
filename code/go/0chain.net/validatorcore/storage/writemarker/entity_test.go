package writemarker_test

import (
	"fmt"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/validatorcore/storage/writemarker"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMarker_GetHashData(t *testing.T) {
	wm, wallet, err := setupEntityTest(t)
	require.NoError(t, err)

	want := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v:%v:%v:%v", "alloc_root", "prev_alloc_root", "file_meta_root", "chain_hash", "alloc_id", "blobber_id", wallet.ClientID, 1, 1, wm.Timestamp)
	got := wm.GetHashData()
	t.Logf("Want: %s. Got: %s", want, got)
	assert.Equal(t, want, got)
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
		name       string
		allocID    string
		allocRoot  string
		publicKey  string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:      "valid",
			allocID:   "alloc_id",
			allocRoot: "alloc_root",
			publicKey: wallet.Keys[0].PublicKey,
		},
		{
			name:       "invalid: wrong Allocation ID",
			allocID:    "id",
			allocRoot:  "alloc_root",
			publicKey:  wallet.Keys[0].PublicKey,
			wantErr:    true,
			wantErrMsg: "Invalid write marker. Allocation ID mismatch",
		},
		{
			name:       "invalid: wrong Allocation Root",
			allocID:    "alloc_id",
			allocRoot:  "root",
			publicKey:  wallet.Keys[0].PublicKey,
			wantErr:    true,
			wantErrMsg: "Invalid write marker. Allocation root mismatch.",
		},
		{
			name:       "invalid: wrong Public Key",
			allocID:    "alloc_id",
			allocRoot:  "alloc_root",
			publicKey:  "1",
			wantErr:    true,
			wantErrMsg: "Invalid write marker. Write marker is not from owner",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := wm.Verify(tt.allocID, tt.allocRoot, tt.publicKey)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
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
		FileMetaRoot:           "file_meta_root",
		PreviousAllocationRoot: "prev_alloc_root",
		AllocationID:           "alloc_id",
		Size:                   int64(1),
		BlobberID:              "blobber_id",
		Timestamp:              common.Now(),
		ChainHash:              "chain_hash",
		ChainSize:              int64(1),
	}

	// TODO: why the config param is not used here?
	sigSch := zcncrypto.NewSignatureScheme("bls0chain")
	wallet, err := sigSch.GenerateKeys()
	if err != nil {
		return wm, wallet, err
	}

	wm.ClientID = wallet.ClientID
	sig, err := sigSch.Sign(encryption.Hash(wm.GetHashData()))
	if err != nil {
		return wm, wallet, err
	}

	wm.Signature = sig
	return wm, wallet, nil
}
