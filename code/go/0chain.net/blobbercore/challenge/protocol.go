package challenge

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"

	"go.uber.org/zap"
)

const VALIDATOR_URL = "/v1/storage/challenge/new"

type ChallengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	ValidationTickets []*ValidationTicket `json:"validation_tickets"`
}

func (cr *ChallengeEntity) SubmitChallengeToBC(ctx context.Context) (*transaction.Transaction, error) {

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	sn := &ChallengeResponse{}
	sn.ChallengeID = cr.ChallengeID
	sn.ValidationTickets = cr.ValidationTickets

	snBytes, err := json.Marshal(sn)
	if err != nil {
		return nil, err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.CHALLENGE_RESPONSE, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed submitting challenge to the mining network", zap.String("err:", err.Error()))
		return nil, err
	}

	Logger.Info("Verifying challenge response to blockchain.", zap.String("txn", txn.Hash), zap.String("challenge_id", cr.ChallengeID))
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
	cr.StatusMessage = err.Error()
	cr.UpdatedAt = time.Now().UTC()

	if err := cr.Save(ctx); err != nil {
		Logger.Error("[challenge]db: ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
	}
}

// LoadValidationTickets load validation tickets
func (cr *ChallengeEntity) LoadValidationTickets(ctx context.Context) error {
	if len(cr.Validators) == 0 {
		cr.StatusMessage = "No validators assigned to the challenge"
		cr.UpdatedAt = time.Now().UTC()

		if err := cr.Save(ctx); err != nil {
			Logger.Error("[challenge]db: ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
		}
		return common.NewError("no_validators", "No validators assigned to the challenge")
	}

	allocationObj, err := allocation.GetAllocationByID(ctx, cr.AllocationID)
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
		blockNum = r.Int63n(rootRef.NumBlocks)
		blockNum = blockNum + 1
		cr.BlockNum = blockNum
	} else {
		err = common.NewError("allocation_is_blank", "Got a challenge for a blank allocation")
		cr.ErrorChallenge(ctx, err)
		return err
	}

	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}

	Logger.Info("[challenge]rand: ", zap.Any("rootRef.NumBlocks", rootRef.NumBlocks), zap.Any("blockNum", blockNum), zap.Any("challenge_id", cr.ChallengeID), zap.Any("random_seed", cr.RandomNumber))
	objectPath, err := reference.GetObjectPath(ctx, cr.AllocationID, blockNum)
	if err != nil {
		cr.ErrorChallenge(ctx, err)
		return err
	}
	cr.RefID = objectPath.RefID
	cr.RespondedAllocationRoot = allocationObj.AllocationRoot
	cr.ObjectPath = objectPath

	postData := make(map[string]interface{})
	postData["challenge_id"] = cr.ChallengeID
	postData["object_path"] = objectPath
	markersArray := make([]map[string]interface{}, 0)
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
		inputData.ChunkSize = objectPath.ChunkSize

		maxNumBlocks := 1024

		// the file is too small, some of 1024 blocks is not filled
		if objectPath.Size < objectPath.ChunkSize {
			merkleChunkSize := objectPath.ChunkSize / 1024
			maxNumBlocks = int(math.Ceil(float64(objectPath.Size) / float64(merkleChunkSize)))
		}

		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockoffset := r.Intn(maxNumBlocks)
		blockData, mt, err := filestore.GetFileStore().GetFileBlockForChallenge(cr.AllocationID, inputData, blockoffset)

		if err != nil {
			cr.ErrorChallenge(ctx, err)
			return common.NewError("blockdata_not_found", err.Error())
		}
		postData["data"] = []byte(blockData)
		postData["merkle_path"] = mt.GetPathByIndex(blockoffset)
		postData["chunk_size"] = objectPath.ChunkSize
	}

	postDataBytes, err := json.Marshal(postData)
	if err != nil {
		Logger.Error("[db]form: " + err.Error())
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
			Logger.Info("[challenge]post: ", zap.Any("error", err.Error()))
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		var validationTicket ValidationTicket
		err = json.Unmarshal(resp, &validationTicket)
		if err != nil {
			Logger.Error("[challenge]resp: ", zap.String("validator", validator.ID), zap.Any("resp", string(resp)), zap.Any("error", err.Error()))
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		Logger.Info("[challenge]resp: Got response from the validator.", zap.Any("validator_response", validationTicket))
		verified, err := validationTicket.VerifySign()
		if err != nil || !verified {
			Logger.Error("[challenge]ticket: Validation ticket from validator could not be verified.", zap.String("validator", validator.ID))
			delete(responses, validator.ID)
			cr.ValidationTickets[i] = nil
			continue
		}
		responses[validator.ID] = validationTicket
		cr.ValidationTickets[i] = &validationTicket
	}

	numSuccess := 0
	numFailure := 0

	numValidatorsResponded := 0
	for _, vt := range cr.ValidationTickets {
		if vt != nil {
			if vt.Result {
				numSuccess++
			} else {
				numFailure++
			}
			numValidatorsResponded++
		}
	}

	Logger.Info("[challenge]validator response stats", zap.Any("challenge_id", cr.ChallengeID), zap.Any("validator_responses", responses))
	if numSuccess > (len(cr.Validators)/2) || numFailure > (len(cr.Validators)/2) || numValidatorsResponded == len(cr.Validators) {
		if numSuccess > (len(cr.Validators) / 2) {
			cr.Result = ChallengeSuccess
		} else {
			cr.Result = ChallengeFailure

			Logger.Error("[challenge]validate: ", zap.String("challenge_id", cr.ChallengeID), zap.Any("block_num", cr.BlockNum), zap.Any("object_path", objectPath))
		}

		cr.Status = Processed
		cr.UpdatedAt = time.Now().UTC()
	} else {
		cr.ErrorChallenge(ctx, common.NewError("no_consensus_challenge", "No Consensus on the challenge result. Erroring out the challenge"))
		return common.NewError("no_consensus_challenge", "No Consensus on the challenge result. Erroring out the challenge")
	}

	return cr.Save(ctx)
}

func (cr *ChallengeEntity) CommitChallenge(ctx context.Context, verifyOnly bool) error {

	if len(cr.LastCommitTxnIDs) > 0 {
		for _, lastTxn := range cr.LastCommitTxnIDs {
			Logger.Info("[challenge]commit: Verifying the transaction : " + lastTxn)
			t, err := transaction.VerifyTransaction(lastTxn, chain.GetServerChain())
			if err == nil {
				cr.Status = Committed
				cr.StatusMessage = t.TransactionOutput
				cr.CommitTxnID = t.Hash
				if err := cr.Save(ctx); err != nil {
					Logger.Error("[challenge]db: ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
				}
				FileChallenged(ctx, cr.RefID, cr.Result, cr.CommitTxnID)
				return nil
			}
			Logger.Error("[challenge]trans: Error verifying the txn from BC."+lastTxn, zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
		}
	}

	if verifyOnly {
		return nil
	}

	t, err := cr.SubmitChallengeToBC(ctx)
	if err != nil {
		if t != nil {
			cr.CommitTxnID = t.Hash
			cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
		}
		cr.ErrorChallenge(ctx, err)
		Logger.Error("[challenge]submit: Error while submitting challenge to BC.", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
	} else {
		cr.Status = Committed
		cr.StatusMessage = t.TransactionOutput
		cr.CommitTxnID = t.Hash
		cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
		cr.UpdatedAt = time.Now().UTC()
	}
	err = cr.Save(ctx)
	FileChallenged(ctx, cr.RefID, cr.Result, cr.CommitTxnID)
	return err
}
