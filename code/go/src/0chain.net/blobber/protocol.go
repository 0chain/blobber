package blobber

import (
	"0chain.net/chain"
)

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	VerifyAllocationTransaction()
	VerifyBlobberTransaction()
	VerifyMarker()
	CollectMarker()
	RedeemMarker()
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain *chain.Chain
}

//ProtocolImpl - singleton for the protocol implementation
var ProtocolImpl StorageProtocol

func GetProtocolImpl() StorageProtocol {
	return ProtocolImpl
}

//SetupProtocol - sets up the protocol for the chain
func SetupProtocol(c *chain.Chain) {
	ProtocolImpl = &StorageProtocolImpl{ServerChain: c}
}

func (sp *StorageProtocolImpl) VerifyAllocationTransaction() {

}

func (sp *StorageProtocolImpl) VerifyBlobberTransaction() {

}

func (sp *StorageProtocolImpl) VerifyMarker() {

}

func (sp *StorageProtocolImpl) CollectMarker() {

}

func (sp *StorageProtocolImpl) RedeemMarker() {

}
