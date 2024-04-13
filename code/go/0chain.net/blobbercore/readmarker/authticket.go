package readmarker

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

const (
	NinetyDays = common.Timestamp(90 * 24 * time.Hour)
)

// swagger:model AuthTicke
type AuthTicket struct {
	ClientID        string           `json:"client_id"`
	OwnerID         string           `json:"owner_id"`
	AllocationID    string           `json:"allocation_id"`
	FilePathHash    string           `json:"file_path_hash"`
	ActualFileHash  string           `json:"actual_file_hash"`
	FileName        string           `json:"file_name"`
	RefType         string           `json:"reference_type"`
	Expiration      common.Timestamp `json:"expiration"`
	Timestamp       common.Timestamp `json:"timestamp"`
	ReEncryptionKey string           `json:"re_encryption_key"`
	Signature       string           `json:"signature"`
	Encrypted       bool             `json:"encrypted"`
}

func (rm *AuthTicket) GetHashData() string {
	hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v:%v:%v:%v:%v",
		rm.AllocationID,
		rm.ClientID,
		rm.OwnerID,
		rm.FilePathHash,
		rm.FileName,
		rm.RefType,
		rm.ReEncryptionKey,
		rm.Expiration,
		rm.Timestamp,
		rm.ActualFileHash,
		rm.Encrypted,
	)
	return hashData
}

func (authToken *AuthTicket) Verify(allocationObj *allocation.Allocation, clientID string) error {
	if authToken.AllocationID != allocationObj.ID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Allocation id mismatch")
	}
	if authToken.ClientID != "" && authToken.ClientID != clientID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Client ID mismatch")
	}

	if authToken.Expiration > 0 {
		if authToken.Expiration < authToken.Timestamp || authToken.Expiration <= common.Now() {
			return common.NewError("invalid_parameters", "Invalid auth ticket. Expired ticket")
		}
	} else { // check for default 90 days expiration time
		if authToken.Timestamp+NinetyDays <= common.Now() {
			return common.NewError("invalid_parameters", "Authticket expired")
		}
	}

	if authToken.OwnerID != allocationObj.OwnerID {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Owner ID mismatch")
	}
	if authToken.Timestamp > (common.Now() + 2) {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Timestamp in future")
	}

	hashData := authToken.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(allocationObj.OwnerPublicKey, authToken.Signature, signatureHash)
	if err != nil || !sigOK {
		return common.NewError("invalid_parameters", "Invalid auth ticket. Signature verification failed")
	}

	return nil
}
