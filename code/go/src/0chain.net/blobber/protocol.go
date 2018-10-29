package blobber

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"0chain.net/util"
	"golang.org/x/crypto/sha3"

	"0chain.net/badgerdbstore"
	"0chain.net/encryption"

	"0chain.net/chain"
	"0chain.net/common"
	. "0chain.net/logging"
	"0chain.net/node"
	"0chain.net/transaction"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

const CHUNK_SIZE = 64 * 1024

type ChallengeResponse struct {
	Data        []byte                   `json:"data_bytes"`
	WriteMarker *writemarker.WriteMarker `json:"write_marker"`
	MerkleRoot  string                   `json:"merkle_root"`
	MerklePath  *util.MTPath             `json:"merkle_path"`
	CloseTxnID  string                   `json:"close_txn_id"`
}

//StorageProtocol - interface for the storage protocol
type StorageProtocol interface {
	RegisterBlobber() (string, error)
	VerifyAllocationTransaction()
	VerifyBlobberTransaction(txn_hash string, clientID string) (*transaction.StorageConnection, error)
	VerifyMarker(wm *writemarker.WriteMarker, sc *transaction.StorageConnection, clientPublicKey string) error
	RedeemMarker(wm *writemarker.WriteMarkerEntity)
	GetChallengeResponse(allocationID string, dataID string, blockNum int64, objectsPath string) (string, error)
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

func (sp *StorageProtocolImpl) sendChallengeTransaction(txn *transaction.Transaction) bool {
	transaction.SendTransactionSync(txn, sp.ServerChain)

	verifyRetries := 0
	txnVerified := false
	var err error
	for verifyRetries < transaction.MAX_TXN_RETRIES {
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		t, err := transaction.VerifyTransaction(txn.Hash, sp.ServerChain)
		if err == nil {
			txnVerified = true
			Logger.Info("Transaction for challenge response is accepted and verified", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
			break
		}
		verifyRetries++
	}

	if !txnVerified {
		Logger.Error("Error verifying the challenge response transaction", zap.Any("err:", err), zap.String("txn.Hash", txn.Hash))
		return false
	}
	return true
}

func (sp *StorageProtocolImpl) SubmitChallenge(path string, wmEntity *writemarker.WriteMarkerEntity, blockNum int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		Logger.Error("Error opening the file in respoding to challenge", zap.String("path", path))
		return "", err
	}
	defer file.Close()

	response := &ChallengeResponse{}
	response.CloseTxnID = wmEntity.CloseTxnID

	bytesBuf := bytes.NewBuffer(make([]byte, 0))
	merkleHash := sha3.New256()
	tReader := io.TeeReader(file, merkleHash)
	merkleLeaves := make([]util.Hashable, 0)
	numRead := int64(0)
	counter := int64(0)
	for true {
		n, err := io.CopyN(bytesBuf, tReader, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			Logger.Error("Error generating merkle tree for the file in respoding to challenge", zap.String("path", path))
			return "", err
		}
		//Logger.Info("reading bytes from file", zap.Int64("read", n))
		numRead += n
		//Logger.Info("hex.EncodeToString(merkleHash.Sum(nil))", zap.String("hash", hex.EncodeToString(merkleHash.Sum(nil))))
		merkleLeaves = append(merkleLeaves, util.NewStringHashable(hex.EncodeToString(merkleHash.Sum(nil))))
		counter++
		if counter == blockNum {
			//Logger.Info("Length of bytes read : ", zap.Int("length", len(bytesBuf.Bytes())))
			//Logger.Info("Hash of bytes read : ", zap.String("hash", encryption.Hash(bytesBuf.Bytes())))
			tmp := make([]byte, len(bytesBuf.Bytes()))
			copy(tmp, bytesBuf.Bytes())
			response.Data = tmp
		}
		merkleHash.Reset()
		bytesBuf.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}

	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)

	response.MerkleRoot = mt.GetRoot()
	response.MerklePath = mt.GetPathByIndex(int(blockNum) - 1)
	response.WriteMarker = wmEntity.WM

	

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CHALLENGE_RESPONSE
	scData.InputArgs = response

	challengeRetries := 0
	lastChallengeTxn:=""
	for (challengeRetries < transaction.MAX_TXN_RETRIES) {
		txn := transaction.NewTransactionEntity()
		txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
		txn.Value = 0
		txn.TransactionType = transaction.TxnTypeSmartContract
		txnBytes, err := json.Marshal(scData)
		if err != nil {
			Logger.Error("Error encoding challenge input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
			return "", err
		}
		txn.TransactionData = string(txnBytes)

		err = txn.ComputeHashAndSign()
		if err != nil {
			Logger.Error("Signing Failed during sending challenge response connection to the miner. ", zap.String("err:", err.Error()))
			return "", err
		}
		if (sp.sendChallengeTransaction(txn)) {
			return txn.Hash, nil
		}
		challengeRetries++
		time.Sleep(transaction.SLEEP_BETWEEN_RETRIES)
		lastChallengeTxn = txn.Hash
	}
	

	

	return lastChallengeTxn, common.NewError("challenge_failed", "Challenge failed to verify after max retries")
}

func (sp *StorageProtocolImpl) GetChallengeResponse(allocationID string, dataID string, blockNum int64, objectsPath string) (string, error) {

	dbstore := badgerdbstore.GetStorageProvider()
	wmEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	wmEntity.AllocationID = allocationID
	wmEntity.WM = &writemarker.WriteMarker{}
	wmEntity.WM.DataID = dataID

	err := dbstore.Read(common.GetRootContext(), wmEntity.GetKey(), wmEntity)
	if err != nil {
		return "", common.NewError("invalid_challenge_parameters", "Could not locate the data id")
	}

	numBlocks := int64(wmEntity.ContentSize/(CHUNK_SIZE)) + 1
	if numBlocks < blockNum || blockNum <= 0 {
		return "", common.NewError("invalid_challenge_parameters", "Invalid block number. Data does not have that many blocks.")
	}

	dirPath, dirFileName := getFilePathFromHash(wmEntity.ContentHash)
	path := filepath.Join(objectsPath, dirPath, dirFileName)

	file, err := os.Open(path)
	if err != nil {
		return "", common.NewError("file_not_found", "Could not find the object from the storage")
	}
	defer file.Close()
	txn, err := sp.SubmitChallenge(path, wmEntity, blockNum)
	return txn, err
}

func (sp *StorageProtocolImpl) RegisterBlobber() (string, error) {
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

func (sp *StorageProtocolImpl) VerifyAllocationTransaction() {

}

func (sp *StorageProtocolImpl) VerifyBlobberTransaction(txn_hash string, clientID string) (*transaction.StorageConnection, error) {
	if len(txn_hash) == 0 {
		return nil, common.NewError("open_connection_txn_invalid", "Open connection Txn is blank. ")
	}

	t, err := transaction.VerifyTransaction(txn_hash, sp.ServerChain)
	if err != nil {
		return nil, common.NewError("open_connection_txn_invalid", "Open connection Txn could not be found. "+err.Error())
	}
	if t.ClientID != clientID {
		return nil, common.NewError("open_connection_txn_invalid", "Open connection Txn should be same client as the write marker. "+err.Error())
	}
	var storageConnection transaction.StorageConnection
	err = json.Unmarshal([]byte(t.TransactionOutput), &storageConnection)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error", "Error decoding the transaction output."+err.Error())
	}
	foundBlobber := false
	for _, blobberConnection := range storageConnection.BlobberData {
		if blobberConnection.BlobberID == node.Self.ID {
			foundBlobber = true
			break
		}
	}
	if !foundBlobber {
		return nil, common.NewError("invalid_blobber", "Blobber is not part of the open connection transaction")
	}
	return &storageConnection, nil
}

func (sp *StorageProtocolImpl) VerifyMarker(wm *writemarker.WriteMarker, storageConnection *transaction.StorageConnection, clientPublicKey string) error {

	if wm == nil {
		return common.NewError("no_write_marker", "No Write Marker was found")
	} else {
		if wm.BlobberID != node.Self.ID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
		}
		if len(wm.IntentTransactionID) == 0 {
			return common.NewError("write_marker_validation_failed", "Write Marker has no valid intent transaction")
		}

		dbstore := badgerdbstore.GetStorageProvider()
		wmEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
		wmEntity.AllocationID = sp.AllocationID
		wmEntity.WM = wm
		

		errWmRead := dbstore.Read(common.GetRootContext(), wmEntity.GetKey(), wmEntity)
		if errWmRead == nil && wmEntity.Status != writemarker.Failed {
			return common.NewError("write_marker_validation_failed", "Duplicate write marker. Validation failed")
		}
		
		txnoutput := storageConnection
		var err error
		if txnoutput == nil {
			txnoutput, err = sp.VerifyBlobberTransaction(wm.IntentTransactionID, wm.ClientID)
			if err != nil {
				return err
			}
		}

		Logger.Info("Transaction out received.", zap.Any("storage_connection", txnoutput))

		foundDataID := false
		var wmBlobberConnection *transaction.StorageConnectionBlobber = nil

		for _, blobberConnection := range txnoutput.BlobberData {
			if blobberConnection.BlobberID == node.Self.ID {
				if blobberConnection.DataID == wm.DataID {
					foundDataID = true
					wmBlobberConnection = &blobberConnection
					break
				}
			}
		}
		if !foundDataID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the data being uploaded")
		}
		if txnoutput.AllocationID != sp.AllocationID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
		}
		if wmBlobberConnection != nil && wmBlobberConnection.OpenConnectionTxn != wm.IntentTransactionID {
			return common.NewError("write_marker_validation_failed", "Write Marker is not for the same intent transaction")
		}
		if wmBlobberConnection != nil && len(txnoutput.ClientPublicKey) == 0 && len(clientPublicKey) == 0 {
			return common.NewError("client_public_not_found", "Could not get the public key of the client")
		}
		
		clientPubKey := txnoutput.ClientPublicKey
		if len(clientPubKey) == 0 {
			clientPubKey = clientPublicKey
		}
		merkleRoot := wm.MerkleRoot
		if len(wm.MerkleRoot) == 0 {
			merkleRoot = "null"
		}
		hashData := fmt.Sprintf("%v:%v:%v:%v:%v:%v", wm.DataID, merkleRoot, wm.IntentTransactionID, wm.BlobberID, wm.Timestamp, wm.ClientID)
		signatureHash := encryption.Hash(hashData)
		Logger.Info("Computed the hash for verifying wm signature. ", zap.String("hashdata", hashData), zap.String("hash", signatureHash))
		sigOK, err := encryption.Verify(clientPubKey, wm.Signature, signatureHash)
		if err != nil {
			return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
		}
		if !sigOK {
			return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
		}
	}
	return nil
}

func (sp *StorageProtocolImpl) RedeemMarker(wm *writemarker.WriteMarkerEntity) {
	txn := transaction.NewTransactionEntity()

	sn := &transaction.CloseConnection{}
	sn.DataID = wm.WM.DataID
	sn.MerkleRoot = wm.MerkleRoot
	sn.WriteMarker = *wm.WM
	sn.Size = wm.ContentSize

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CLOSE_CONNECTION_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Error encoding sc input. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Error("Signing Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	transaction.SendTransactionSync(txn, sp.ServerChain)
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, sp.ServerChain)
	if err != nil {
		Logger.Error("Error verifying the commit transaction", zap.String("err:", err.Error()))
		wm.Status = writemarker.Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		wm.Write(common.GetRootContext())
		return
	}
	wm.Status = writemarker.Committed
	wm.StatusMessage = t.TransactionOutput
	wm.CloseTxnID = t.Hash
	wm.Write(common.GetRootContext())

	debugEntity := writemarker.Provider()
	badgerdbstore.GetStorageProvider().Read(common.GetRootContext(), wm.GetKey(), debugEntity)
	Logger.Info("Debugging to see if saving was successful", zap.Any("wm", debugEntity))
	return
}
