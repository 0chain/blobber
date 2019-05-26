package challenge

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/writemarker"
	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/filestore"
	"0chain.net/core/transaction"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/util"
	."0chain.net/core/logging"

	"go.uber.org/zap"
)

const VALIDATOR_URL = "/v1/storage/challenge/new"

type ChallengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	ValidationTickets []*ValidationTicket `json:"validation_tickets"`
}

func (cr *ChallengeEntity) SubmitChallengeToBC(ctx context.Context) (*transaction.Transaction, error) {

	txn := transaction.NewTransactionEntity()

	sn := &ChallengeResponse{}
	sn.ChallengeID = cr.ChallengeID
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
	Logger.Info("Submitting challenge response to blockchain.", zap.String("txn", txn.Hash), zap.String("challenge_id", cr.ChallengeID))
	transaction.SendTransaction(txn, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the challenge response transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash), zap.String("challenge_id", cr.ChallengeID))
		return txn, err
	}
	Logger.Info("Challenge committed and accepted", zap.Any("txn.hash", t.Hash), zap.Any("txn.output", t.TransactionOutput), zap.String("challenge_id", cr.ChallengeID))
	return t, nil
}

func (cr *ChallengeEntity) ErrorChallenge(ctx context.Context, err error) {
	cr.Status = Error
	cr.StatusMessage = err.Error()
	cr.Save(ctx)
}

func (cr *ChallengeEntity) SendDataBlockToValidators(ctx context.Context) error {
	if len(cr.Validators) == 0 {
		cr.Status = Failed
		cr.StatusMessage = "No validators assigned to the challange"
		cr.Save(ctx)
		return common.NewError("no_validators", "No validators assigned to the challange")
	}
	if len(cr.LastCommitTxnIDs) > 0 {
		for _, lastTxn := range cr.LastCommitTxnIDs {
			Logger.Info("Verifying the transaction : " + lastTxn)
			t, err := transaction.VerifyTransaction(lastTxn, chain.GetServerChain())
			if err == nil {
				cr.Status = Committed
				cr.StatusMessage = t.TransactionOutput
				cr.CommitTxnID = t.Hash
				cr.Save(ctx)
				return nil
			}
			Logger.Error("Error verifying the txn from BC."+lastTxn, zap.String("challenge_id", cr.ChallengeID))
		}

	}

	allocationObj, err := allocation.VerifyAllocationTransaction(ctx, cr.AllocationID, true)
	if err != nil {
		return err
	}

	wms, err := writemarker.GetWriteMarkersInRange(ctx, cr.AllocationID, cr.AllocationRoot, allocationObj.AllocationRoot)
	if err != nil {
		return err
	}
	if len(wms) == 0 {
		return common.NewError("write_marker_not_found", "Could find the writemarker for the given allocation root on challenge")
	}
	
	rootRef, err := reference.GetReference(ctx, cr.AllocationID, "/")
	blockNum := int64(0)
	if rootRef.NumBlocks > 0 {
		r := rand.New(rand.NewSource(cr.RandomNumber))
		//rand.Seed(cr.RandomNumber)
		blockNum = r.Int63n(rootRef.NumBlocks)
		blockNum = blockNum + 1
	} else {
		Logger.Error("Got a challenge for a blank allocation")
	}

	cr.BlockNum = blockNum
	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}
	Logger.Info("blockNum for challenge", zap.Any("rootRef.NumBlocks", rootRef.NumBlocks), zap.Any("blockNum", blockNum), zap.Any("challenge_id", cr.ChallengeID), zap.Any("random_seed", cr.RandomNumber))
	objectPath, err := reference.GetObjectPath(ctx, cr.AllocationID, blockNum)
	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}

	postData := make(map[string]interface{})
	postData["challenge_id"] = cr.ChallengeID
	postData["object_path"] = objectPath
	markersArray := make([]map[string]interface{},0)
	for _, wm := range wms {
		markersMap := make(map[string]interface{})
		markersMap["write_marker"] = wm.WM
		markersMap["client_key"] = wm.ClientPublicKey
		markersArray = append(markersArray, markersMap)
	}
	postData["write_markers"] = markersArray
	

	if blockNum > 0 {
		if objectPath.Meta["type"] != reference.FILE {
			Logger.Info("Block number to be challenged for file:", zap.Any("block", objectPath.FileBlockNum), zap.Any("meta", objectPath.Meta), zap.Any("obejct_path", objectPath))
			err = common.NewError("invalid_object_path", "Object path was not for a file")
			cr.ErrorChallenge(ctx, err)
			return err
		}

		inputData := &filestore.FileInputData{}
		inputData.Name = objectPath.Meta["name"].(string)
		inputData.Path = objectPath.Meta["path"].(string)
		inputData.Hash = objectPath.Meta["content_hash"].(string)
		blockData, err := filestore.GetFileStore().GetFileBlock(cr.AllocationID, inputData, objectPath.FileBlockNum)
		mt, err := filestore.GetFileStore().GetMerkleTreeForFile(cr.AllocationID, inputData) 
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
	if cr.ValidationTickets == nil {
		cr.ValidationTickets = make([]*ValidationTicket, len(cr.Validators))
	}
	for i, validator := range cr.Validators {
		if cr.ValidationTickets[i] != nil {
			exisitingVT := cr.ValidationTickets[i]
			if len(exisitingVT.Signature) > 0 && exisitingVT.ChallengeID == cr.ChallengeID {
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

	if numSuccess > (len(cr.Validators)/2) || numFailure > (len(cr.Validators)/2) || len(cr.ValidationTickets) == len(cr.Validators) {
		if numSuccess > (len(cr.Validators) / 2) {
			cr.Result = ChallengeSuccess
		} else {
			cr.Result = ChallengeFailure
			Logger.Error("Challenge failed by the validators", zap.Any("block_num", cr.BlockNum), zap.Any("object_path", objectPath), zap.Any("challenge", cr))
		}
		t, err := cr.SubmitChallengeToBC(ctx)
		if err != nil {
			if t != nil {
				cr.CommitTxnID = t.Hash
				cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
			}
			cr.ErrorChallenge(ctx, err)
		} else {
			cr.Status = Committed
			cr.StatusMessage = t.TransactionOutput
			cr.CommitTxnID = t.Hash
			cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
		}
	} else {
		cr.ErrorChallenge(ctx, common.NewError("no_consensus_challenge", "No Consensus on the challenge result. Erroring out the challenge"))
		return common.NewError("no_consensus_challenge", "No Consensus on the challenge result. Erroring out the challenge")
	}

	cr.Save(ctx)
	return nil
}
