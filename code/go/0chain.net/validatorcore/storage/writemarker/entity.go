package writemarker

import (
	"encoding/hex"
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

type WriteMarkerEntity struct {
	ClientPublicKey string       `json:"client_key"`
	WM              *WriteMarker `json:"write_marker"`
}

type WriteMarker struct {
	AllocationRoot         string           `json:"allocation_root"`
	PreviousAllocationRoot string           `json:"prev_allocation_root"`
	FileMetaRoot           string           `json:"file_meta_root"`
	AllocationID           string           `json:"allocation_id"`
	Size                   int64            `json:"size"`
	ChainSize              int64            `json:"chain_size"`
	ChainHash              string           `json:"chain_hash"`
	BlobberID              string           `json:"blobber_id"`
	Timestamp              common.Timestamp `json:"timestamp"`
	ClientID               string           `json:"client_id"`
	Signature              string           `json:"signature"`
}

func (wm *WriteMarker) GetHashData() string {
	var hashData string
	if wm.ChainHash != "" {
		hashData = fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%d:%d",
			wm.AllocationRoot, wm.PreviousAllocationRoot,
			wm.FileMetaRoot, wm.ChainHash, wm.AllocationID, wm.BlobberID,
			wm.ClientID, wm.Size, wm.ChainSize, wm.Timestamp)
	} else {
		hashData = fmt.Sprintf("%s:%s:%s:%s:%s:%s:%d:%d",
			wm.AllocationRoot, wm.PreviousAllocationRoot,
			wm.FileMetaRoot, wm.AllocationID, wm.BlobberID,
			wm.ClientID, wm.Size, wm.Timestamp)
	}
	return hashData
}

func (wm *WriteMarker) VerifySignature(clientPublicKey string) bool {
	hashData := wm.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, wm.Signature, signatureHash)
	if err != nil {
		return false
	}
	if !sigOK {
		return false
	}
	return true
}

func (wm *WriteMarker) Verify(allocationID, allocationRoot, clientPublicKey string) error {
	if wm.AllocationID != allocationID {
		return common.NewError("challenge_validation_failed", "Invalid write marker. Allocation ID mismatch")
	}

	if wm.AllocationRoot != allocationRoot {
		return common.NewError("challenge_validation_failed", "Invalid write marker. Allocation root mismatch. Got "+wm.AllocationRoot+" expected: "+allocationRoot)
	}
	clientKeyBytes, _ := hex.DecodeString(clientPublicKey)
	if wm.ClientID != encryption.Hash(clientKeyBytes) {
		return common.NewError("challenge_validation_failed", "Invalid write marker. Write marker is not from owner")
	}

	if !wm.VerifySignature(clientPublicKey) {
		return common.NewError("challenge_validation_failed", "Invalid write marker. Write marker is not from owner. Signature validation failure")
	}
	return nil
}
