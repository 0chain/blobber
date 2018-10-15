package blobber

import (
	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/node"
	"0chain.net/writemarker"
)

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	VerifyAllocationTransaction()
	VerifyBlobberTransaction()
	VerifyMarker() error
	RedeemMarker()
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain  *chain.Chain
	AllocationID string
	IntentTxnID  string
	DataID       string
	WriteMarker  *writemarker.WriteMarker
}

//ProtocolImpl - singleton for the protocol implementation
var ProtocolImpl StorageProtocol

func GetProtocolImpl(allocationID string, intentTxn string, dataID string, wm *writemarker.WriteMarker) StorageProtocol {
	return &StorageProtocolImpl{
		ServerChain:  chain.GetServerChain(),
		AllocationID: allocationID,
		IntentTxnID:  intentTxn,
		DataID:       dataID,
		WriteMarker:  wm}
}

//SetupProtocol - sets up the protocol for the chain
func SetupProtocol(c *chain.Chain) {
	ProtocolImpl = &StorageProtocolImpl{ServerChain: c}
}

func (sp *StorageProtocolImpl) VerifyAllocationTransaction() {

}

func (sp *StorageProtocolImpl) VerifyBlobberTransaction() {

}

func (sp *StorageProtocolImpl) VerifyMarker() error {
	wm := sp.WriteMarker
	if wm == nil {
		return common.NewError("no_write_marker", "No Write Marker was found")
	} else {
		if wm.BlobberID != node.Self.ID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
		}
		if wm.DataID != sp.DataID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the data being uploaded")
		}
		if wm.IntentTransactionID != sp.IntentTxnID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same intent transaction")
		}
	}
	return nil
}

func (sp *StorageProtocolImpl) RedeemMarker() {

}
