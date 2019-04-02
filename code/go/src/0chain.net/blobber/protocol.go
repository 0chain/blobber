package blobber

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/allocation"
	"0chain.net/chain"
	"0chain.net/challenge"
	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	. "0chain.net/logging"
	"0chain.net/node"
	"0chain.net/readmarker"
	"0chain.net/reference"
	"0chain.net/stats"
	"0chain.net/transaction"
	"0chain.net/writemarker"

	"go.uber.org/zap"
)

const CHUNK_SIZE = reference.CHUNK_SIZE

// type ChallengeResponse struct {
// 	Data        []byte                   `json:"data_bytes"`
// 	WriteMarker *writemarker.WriteMarker `json:"write_marker"`
// 	MerkleRoot  string                   `json:"merkle_root"`
// 	MerklePath  *util.MTPath             `json:"merkle_path"`
// 	CloseTxnID  string                   `json:"close_txn_id"`
// }

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	RegisterBlobber(ctx context.Context) (string, error)
	VerifyAllocationTransaction(ctx context.Context, readonly bool) (*allocation.Allocation, error)
	VerifyMarker(ctx context.Context, wm *writemarker.WriteMarker, sa *allocation.Allocation, co *allocation.AllocationChangeCollector) error
	VerifyReadMarker(ctx context.Context, rm *readmarker.ReadMarker, sa *allocation.Allocation) error
	VerifyChallengeRequest(ctx context.Context, challengeID string) (*challenge.ChallengeEntity, error)
}

//StorageProtocolImpl - implementation of the storage protocol
type StorageProtocolImpl struct {
	ServerChain  *chain.Chain
	AllocationID string
}

func GetProtocolImpl(allocationID string) StorageProtocol {
	return &StorageProtocolImpl{
		ServerChain:  chain.GetServerChain(),
		AllocationID: allocationID}
}

func (sp *StorageProtocolImpl) VerifyChallengeRequest(ctx context.Context, challengeID string) (*challenge.ChallengeEntity, error) {
	challengeObj := challenge.Provider().(*challenge.ChallengeEntity)
	challengeObj.ID = challengeID

	err := challengeObj.Read(ctx, challengeObj.GetKey())
	if err != datastore.ErrKeyNotFound {
		return nil, common.NewError("invalid_challenge", "Invalid Challenge id. Already existing.")
	}
	t, err := transaction.VerifyTransaction(challengeID, sp.ServerChain)
	if err != nil {
		return nil, common.NewError("invalid_challenge", "Invalid Challenge id. Challenge not found in blockchain. "+err.Error())
	}

	err = json.Unmarshal([]byte(t.TransactionOutput), challengeObj)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error", "Error decoding the challenge transaction output."+err.Error())
	}

	return challengeObj, nil
}

func (sp *StorageProtocolImpl) VerifyReadMarker(ctx context.Context, rm *readmarker.ReadMarker, sa *allocation.Allocation) error {
	if rm == nil {
		return common.NewError("invalid_read_marker", "No read marker was found")
	}
	if rm.AllocationID != sp.AllocationID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the same allocation")
	}

	if rm.BlobberID != node.Self.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the blobber")
	}

	dbstore := GetMetaDataStore()
	rmEntity := readmarker.Provider().(*readmarker.ReadMarkerEntity)
	rmEntity.LatestRM = &readmarker.ReadMarker{}
	rmEntity.LatestRM.BlobberID = rm.BlobberID
	rmEntity.LatestRM.ClientID = rm.ClientID

	errRmRead := dbstore.Read(ctx, rmEntity.GetKey(), rmEntity)
	if errRmRead != nil && errRmRead != datastore.ErrKeyNotFound {
		return common.NewError("read_marker_db_error", "Could not read from DB. "+errRmRead.Error())
	}
	if errRmRead == nil && rmEntity.LatestRM != nil {
		if rmEntity.LatestRM.ReadCounter >= rm.ReadCounter {
			return common.NewError("invalid_read_marker", "Read marker counter is lesser than previous")
		}
	}

	clientPublicKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)
	if len(clientPublicKey) == 0 || clientPublicKey != rm.ClientPublicKey {
		return common.NewError("read_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || clientID != rm.ClientID {
		return common.NewError("read_marker_validation_failed", "Read Marker clientID does not match request clientID")
	}

	if rm.Timestamp > common.Now() {
		return common.NewError("read_marker_validation_failed", "Timestamp is for future in the read marker")
	}

	hashData := rm.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, rm.Signature, signatureHash)
	if err != nil {
		return common.NewError("read_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("read_marker_validation_failed", "Read marker signature is not valid")
	}
	return nil
}

func (sp *StorageProtocolImpl) VerifyMarker(ctx context.Context, wm *writemarker.WriteMarker, sa *allocation.Allocation, co *allocation.AllocationChangeCollector) error {

	if wm == nil {
		return common.NewError("invalid_write_marker", "No Write Marker was found")
	}
	if wm.PreviousAllocationRoot != sa.AllocationRoot {
		return common.NewError("invalid_write_marker", "Invalid write marker. Prev Allocation root does not match the allocation root on record")
	}
	if wm.BlobberID != node.Self.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
	}
	dbstore := GetMetaDataStore()
	wmEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	wmEntity.WM = wm

	errWmRead := dbstore.Read(ctx, wmEntity.GetKey(), wmEntity)
	if errWmRead == nil && wmEntity.Status != writemarker.Failed {
		return common.NewError("write_marker_validation_failed", "Duplicate write marker. Validation failed")
	}

	if wm.AllocationID != sp.AllocationID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
	}

	if wm.Size != co.Size {
		return common.NewError("write_marker_validation_failed", "Write Marker size does not match the connection size")
	}

	clientPublicKey := ctx.Value(CLIENT_KEY_CONTEXT_KEY).(string)
	if len(clientPublicKey) == 0 {
		return common.NewError("write_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || clientID != wm.ClientID || clientID != co.ClientID || co.ClientID != wm.ClientID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not by the same client who uploaded")
	}

	hashData := wm.GetHashData()
	signatureHash := encryption.Hash(hashData)
	Logger.Info("Computed the hash for verifying wm signature. ", zap.String("hashdata", hashData), zap.String("hash", signatureHash))
	sigOK, err := encryption.Verify(clientPublicKey, wm.Signature, signatureHash)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
	}

	return nil
}

func (sp *StorageProtocolImpl) RegisterBlobber(ctx context.Context) (string, error) {
	nodeBytes, _ := json.Marshal(node.Self)
	transaction.SendPostRequestSync(transaction.REGISTER_CLIENT, nodeBytes, sp.ServerChain)
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	txn := transaction.NewTransactionEntity()

	sn := &transaction.StorageNode{}
	sn.ID = node.Self.GetKey()
	sn.BaseURL = node.Self.GetURLBase()

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.ADD_BLOBBER_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		return "", err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Info("Signing Failed during registering blobber to the mining network", zap.String("err:", err.Error()))
		return "", err
	}
	Logger.Info("Adding blobber to the blockchain.", zap.String("txn", txn.Hash))
	transaction.SendTransaction(txn, sp.ServerChain)
	return txn.Hash, nil
}

func (sp *StorageProtocolImpl) VerifyAllocationTransaction(ctx context.Context, readonly bool) (*allocation.Allocation, error) {
	allocationObj := allocation.Provider().(*allocation.Allocation)
	allocationObj.ID = sp.AllocationID
	err := allocationObj.Read(ctx, allocationObj.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return nil, common.NewError("invalid_allocation", "Invalid Allocation id. Allocation not found")
	}
	if err != nil && err == datastore.ErrKeyNotFound {
		t, err := transaction.VerifyTransaction(sp.AllocationID, sp.ServerChain)
		if err != nil {
			return nil, common.NewError("invalid_allocation", "Invalid Allocation id. Allocation not found in blockchain. "+err.Error())
		}
		var storageAllocation transaction.StorageAllocation
		err = json.Unmarshal([]byte(t.TransactionOutput), &storageAllocation)
		if err != nil {
			return nil, common.NewError("transaction_output_decode_error", "Error decoding the allocation transaction output."+err.Error())
		}
		foundBlobber := false
		for _, blobberConnection := range storageAllocation.Blobbers {
			if blobberConnection.ID == node.Self.ID {
				foundBlobber = true
				allocationObj.AllocationRoot = ""
				allocationObj.BlobberSize = (storageAllocation.Size + int64(len(storageAllocation.Blobbers)-1)) / int64(len(storageAllocation.Blobbers))
				allocationObj.BlobberSizeUsed = 0
				break
			}
		}
		if !foundBlobber {
			return nil, common.NewError("invalid_blobber", "Blobber is not part of the open connection transaction")
		}
		allocationObj.Expiration = storageAllocation.Expiration
		allocationObj.OwnerID = storageAllocation.OwnerID
		allocationObj.OwnerPublicKey = storageAllocation.OwnerPublicKey
		allocationObj.TotalSize = storageAllocation.Size
		allocationObj.UsedSize = storageAllocation.UsedSize
		if !readonly {
			err = allocationObj.Write(ctx)
			if err != nil {
				return nil, common.NewError("allocation_write_error", "Error storing the allocation meta data received from blockchain")
			}
			err = reference.CreateDirRefsIfNotExists(ctx, sp.AllocationID, "/", "", allocationObj.GetEntityMetadata().GetStore())
			if err != nil {
				return nil, common.NewError("root_reference_creation_error", "Error creating the root reference")
			}
			go stats.AddNewAllocationEvent(allocationObj.ID)
		}
	}
	return allocationObj, nil
}
