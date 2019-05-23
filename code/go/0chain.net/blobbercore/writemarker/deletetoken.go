package writemarker

import (
	"fmt"

	"0chain.net/core/common"
	"0chain.net/core/encryption"
)

type DeleteToken struct {
	FilePathHash string           `json:"file_path_hash"`
	FileRefHash  string           `json:"file_ref_hash"`
	AllocationID string           `json:"allocation_id"`
	Size         int64            `json:"size"`
	BlobberID    string           `json:"blobber_id"`
	Timestamp    common.Timestamp `json:"timestamp"`
	ClientID     string           `json:"client_id"`
	Signature    string           `json:"signature"`
}

func (dt *DeleteToken) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", dt.FileRefHash, dt.FilePathHash, dt.AllocationID, dt.BlobberID, dt.ClientID, dt.Size, dt.Timestamp)
	return hashData
}

func (dt *DeleteToken) VerifySignature(clientPubKey string) bool {
	if len(clientPubKey) == 0 {
		return false
	}
	hashData := dt.GetHashData()
	signatureHash := encryption.Hash(hashData)
	ok, err := encryption.Verify(clientPubKey, dt.Signature, signatureHash)
	if err != nil {
		return false
	}
	return ok
}
