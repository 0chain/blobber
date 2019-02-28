package challenge

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"0chain.net/allocation"
	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/filestore"
	"0chain.net/hashmapstore"
	"0chain.net/lock"
	. "0chain.net/logging"
	"0chain.net/reference"
	"0chain.net/transaction"
	"0chain.net/util"
	"0chain.net/writemarker"
)

const VALIDATOR_URL = "/v1/storage/challenge/new"

type ChallengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	ValidationTickets []*ValidationTicket `json:"validation_tickets"`
}

func (cr *ChallengeEntity) SubmitChallengeToBC(ctx context.Context) (*transaction.Transaction, error) {

	txn := transaction.NewTransactionEntity()

	sn := &ChallengeResponse{}
	sn.ChallengeID = cr.ID
	sn.ValidationTickets = cr.ValidationTickets

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CHALLENGE_RESPONSE
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		return nil, err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Info("Signing Failed during submitting challenge to the mining network", zap.String("err:", err.Error()))
		return nil, err
	}
	Logger.Info("Submitting challenge response to blockchain.", zap.String("txn", txn.Hash))
	transaction.SendTransaction(txn, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the challenge response transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		return txn, err
	}
	Logger.Info("Challenge committed and accepted", zap.Any("txn.hash", t.Hash), zap.Any("txn.output", t.TransactionOutput))
	return t, nil
}

func (cr *ChallengeEntity) ErrorChallenge(ctx context.Context, err error) {
	cr.Status = Error
	cr.StatusMessage = err.Error()
	cr.Write(ctx)
}

func (cr *ChallengeEntity) SendDataBlockToValidators(ctx context.Context, fileStore filestore.FileStore) error {
	if len(cr.Validators) == 0 {
		cr.Status = Failed
		cr.StatusMessage = "No validators assigned to the challange"
		cr.Write(ctx)
		return common.NewError("no_validators", "No validators assigned to the challange")
	}
	if len(cr.CommitTxnID) > 0 {
		Logger.Info("Verifying the transaction : " + cr.CommitTxnID)
		t, err := transaction.VerifyTransaction(cr.CommitTxnID, chain.GetServerChain())
		if err == nil {
			cr.Status = Committed
			cr.StatusMessage = t.TransactionOutput
			cr.CommitTxnID = t.Hash
			cr.Write(ctx)
			return nil
		} else {
			Logger.Error("Error verifying the txn from BC." + cr.CommitTxnID)
		}
	}

	wm := writemarker.Provider().(*writemarker.WriteMarkerEntity)
	wm.WM = &writemarker.WriteMarker{}
	wm.WM.AllocationRoot = cr.AllocationRoot
	wm.WM.AllocationID = cr.AllocationID

	err := wm.Read(ctx, wm.GetKey())
	if err != nil {
		return common.NewError("invalid_write_marker", "Write marker not found for the allocation root")
	}
	wmMutex := lock.GetMutex(wm.GetKey())
	wmMutex.Lock()
	defer wmMutex.Unlock()
	if wm.DirStructure == nil {
		wm.WriteAllocationDirStructure(ctx)
	}
	dbStore := hashmapstore.NewStore()
	dbStore.DB = wm.DirStructure
	rootRef, err := reference.GetRootReferenceFromStore(ctx, cr.AllocationID, dbStore)
	rand.Seed(cr.RandomNumber)
	blockNum := rand.Int63n(rootRef.NumBlocks)
	blockNum = blockNum + 1
	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}
	//Logger.Info("Block number to be challenged", zap.Any("block", blockNum))
	objectPath, err := reference.GetObjectPath(ctx, cr.AllocationID, blockNum, dbStore)
	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}

	if objectPath.Meta["type"] != reference.FILE {
		Logger.Info("Block number to be challenged for file:", zap.Any("block", objectPath.FileBlockNum), zap.Any("meta", objectPath.Meta), zap.Any("obejct_path", objectPath))
		err = common.NewError("invalid_object_path", "Object path was not for a file")
		cr.ErrorChallenge(ctx, err)
		return err
	}

	postData := make(map[string]interface{})
	postData["challenge_id"] = cr.ID
	postData["object_path"] = objectPath
	postData["write_marker"] = wm.WM
	postData["client_key"] = wm.ClientPublicKey

	inputData := &filestore.FileInputData{}
	inputData.Name = objectPath.Meta["name"].(string)
	inputData.Path = objectPath.Meta["path"].(string)
	inputData.Hash = objectPath.Meta["content_hash"].(string)
	blockData, err := fileStore.GetFileBlock(cr.AllocationID, inputData, objectPath.FileBlockNum)

	if err != nil {
		dt := allocation.DeleteTokenProvider().(*allocation.DeleteToken)
		dt.FileRefHash = objectPath.Meta["hash"].(string)
		err = dt.Read(ctx, dt.GetKey())
		if err != nil {
			cr.ErrorChallenge(ctx, err)
			return err
		}
		postData["delete_token"] = dt

	} else {
		mt, err := fileStore.GetMerkleTreeForFile(cr.AllocationID, inputData)
		if err != nil {
			cr.ErrorChallenge(ctx, err)
			return err
		}
		postData["data"] = []byte(blockData)
		postData["merkle_path"] = mt.GetPathByIndex(int(objectPath.FileBlockNum) - 1)
	}

	postDataBytes, err := json.Marshal(postData)
	if err != nil {
		Logger.Error("Error in marshalling the post data for validation. " + err.Error())
		cr.ErrorChallenge(ctx, err)
		return err
	}
	responses := make(map[string]ValidationTicket)
	for i, validator := range cr.Validators {
		if cr.ValidationTickets[i] != nil {
			exisitingVT := cr.ValidationTickets[i]
			if len(exisitingVT.Signature) > 0 && exisitingVT.ChallengeID == cr.ID {
				continue
			}
		}
		url := validator.URL + VALIDATOR_URL
		resp, err := util.SendPostRequest(url, postDataBytes, nil)
		if err != nil {
			Logger.Info("Got error from the validator.", zap.Any("error", err.Error()))
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		var validationTicket ValidationTicket
		err = json.Unmarshal(resp, &validationTicket)
		if err != nil {
			Logger.Info("Got error decoding from the validator response .", zap.Any("error", err.Error()))
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		Logger.Info("Got response from the validator.", zap.Any("validator_response", validationTicket))
		verified, err := validationTicket.VerifySign()
		if err != nil || !verified {
			Logger.Info("Validation ticket from validator could not be verified.")
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		responses[validator.ID] = validationTicket
		cr.ValidationTickets[i] = &validationTicket
	}

	numSuccess := 0
	numFailure := 0

	for _, vt := range cr.ValidationTickets {
		if vt != nil {
			if vt.Result {
				numSuccess++
			} else {
				numFailure++
			}
		}
	}
	if numSuccess > (len(cr.Validators)/2) || numFailure > (len(cr.Validators)/2) {
		t, err := cr.SubmitChallengeToBC(ctx)
		if err != nil {
			if t != nil {
				cr.CommitTxnID = t.Hash
			}
			cr.ErrorChallenge(ctx, err)
		} else {
			cr.Status = Committed
			cr.StatusMessage = t.TransactionOutput
			cr.CommitTxnID = t.Hash
		}
	}

	cr.Write(ctx)
	return nil
}
