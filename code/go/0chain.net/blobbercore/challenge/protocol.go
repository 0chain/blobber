package challenge

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	coreTxn "github.com/0chain/gosdk/core/transaction"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
	"github.com/0chain/common/core/util/wmpt"
	sdkUtil "github.com/0chain/gosdk/core/util"
	"github.com/remeh/sizedwaitgroup"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

const VALIDATOR_URL = "/v1/storage/challenge/new"
const ValueNotPresent = "value not present"
const EntityNotFound = "entity not found"

var (
	ErrNoValidator          = errors.New("no validators assigned to the challenge")
	ErrNoConsensusChallenge = errors.New("no_consensus_challenge: No Consensus on the challenge result. Erroring out the challenge")
	ErrInvalidObjectPath    = errors.New("invalid_object_path: Object path was not for a file")
	ErrExpiredCCT           = errors.New("expired challenge completion time")
	ErrValNotPresent        = errors.New("chain responded: " + ValueNotPresent)
	ErrEntityNotFound       = errors.New("chain responded: " + EntityNotFound)
)

type ChallengeResponse struct {
	ChallengeID       string              `json:"challenge_id"`
	ValidationTickets []*ValidationTicket `json:"validation_tickets"`
}

func (cr *ChallengeEntity) CancelChallenge(ctx context.Context, errReason error) {
	cancellation := time.Now()
	db := datastore.GetStore().GetTransaction(ctx)
	deleteChallenge(cr.RoundCreatedAt)
	cr.statusMutex.Lock()
	cr.Status = Cancelled
	cr.Result = ChallengeFailure
	cr.statusMutex.Unlock()
	cr.StatusMessage = errReason.Error()
	cr.UpdatedAt = cancellation.UTC()
	if err := db.Save(cr).Error; err != nil {
		logging.Logger.Error("[challenge]cancel:db ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
	}

	if err := UpdateChallengeTimingCancellation(cr.ChallengeID, common.Timestamp(cancellation.Unix()), errReason); err != nil {
		logging.Logger.Error("[challengetiming]cancellation",
			zap.Any("challenge_id", cr.ChallengeID),
			zap.Time("cancellation", cancellation),
			zap.Error(err))
	}
	logging.Logger.Error("[challenge]canceled", zap.String("challenge_id", cr.ChallengeID), zap.Any("round_created_at", cr.RoundCreatedAt), zap.Error(errReason))
}

// LoadValidationTickets load validation tickets
func (cr *ChallengeEntity) LoadValidationTickets(ctx context.Context) error {
	if len(cr.Validators) == 0 {
		cr.CancelChallenge(ctx, ErrNoValidator)
		return ErrNoValidator
	}

	// Lock allocation changes from happening in handler.CommitWrite function
	// This lock should be unlocked as soon as possible. We should not defer
	// unlocking it as it will be locked for longer time and handler.CommitWrite
	// will fail.
	allocMu := lock.GetMutex(allocation.Allocation{}.TableName(), cr.AllocationID)
	allocMu.RLock()

	allocationObj, err := allocation.Repo.GetAllocationFromDB(ctx, cr.AllocationID)
	if err != nil {
		allocMu.RUnlock()
		cr.CancelChallenge(ctx, err)
		return err
	}

	wms, err := writemarker.GetWriteMarkersInRange(ctx, cr.AllocationID, cr.AllocationRoot, cr.Timestamp, allocationObj.AllocationRoot)
	if err != nil {
		allocMu.RUnlock()
		return err
	}
	if len(wms) == 0 {
		allocMu.RUnlock()
		return common.NewError("write_marker_not_found", "Could find the writemarker for the given allocation root on challenge")
	}

	var (
		postData map[string]any
	)

	if allocationObj.IsStorageV2() {
		postData, err = cr.getPostDataV2(ctx, allocationObj)
	} else {
		postData, err = cr.getPostData(ctx, allocationObj)
	}
	allocMu.RUnlock()
	if err != nil {
		logging.Logger.Error("[challenge]load: ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
		cr.CancelChallenge(ctx, err)
		return err
	}
	postData["challenge_id"] = cr.ChallengeID
	markersArray := make([]map[string]interface{}, 0)
	for _, wm := range wms {
		markersMap := make(map[string]interface{})
		markersMap["write_marker"] = wm.WM
		markersMap["client_key"] = wm.ClientPublicKey
		markersArray = append(markersArray, markersMap)
	}
	postData["write_markers"] = markersArray
	postData["storage_version"] = allocationObj.StorageVersion

	postDataBytes, err := json.Marshal(postData)
	logging.Logger.Info("[challenge]post: ", zap.Any("challenge_id", cr.ChallengeID), zap.Any("post_data_len", len(postDataBytes)/(1024*1024)))
	if err != nil {
		logging.Logger.Error("[db]form: " + err.Error())
		cr.CancelChallenge(ctx, err)
		return err
	}
	responses := make(map[string]ValidationTicket)
	if cr.ValidationTickets == nil {
		cr.ValidationTickets = make([]*ValidationTicket, len(cr.Validators))
	}

	accessMu := sync.RWMutex{}
	updateMapAndSlice := func(validatorID string, i int, vt *ValidationTicket) {
		accessMu.Lock()
		cr.ValidationTickets[i] = vt
		if vt != nil {
			responses[validatorID] = *vt
		} else {
			delete(responses, validatorID)
		}
		accessMu.Unlock()
	}

	numSuccess := 0
	numFailed := 0

	swg := sizedwaitgroup.New(10)
	for i, validator := range cr.Validators {
		if cr.ValidationTickets[i] != nil {
			exisitingVT := cr.ValidationTickets[i]
			if exisitingVT.Signature != "" && exisitingVT.ChallengeID == cr.ChallengeID {
				continue
			}
		}

		url := validator.URL + VALIDATOR_URL

		swg.Add()
		go func(url, validatorID string, i int) {
			defer swg.Done()

			resp, err := util.SendPostRequest(url, postDataBytes, nil)
			if err != nil {
				numFailed++
				logging.Logger.Error("[challenge]post: ", zap.Any("error", err.Error()))
				updateMapAndSlice(validatorID, i, nil)
				return
			}
			var validationTicket ValidationTicket
			err = json.Unmarshal(resp, &validationTicket)
			if err != nil {
				numFailed++
				logging.Logger.Error(
					"[challenge]resp: ",
					zap.String("validator",
						validatorID),
					zap.Any("resp", string(resp)),
					zap.Any("error", err.Error()),
				)
				updateMapAndSlice(validatorID, i, nil)
				return
			}

			if validationTicket.ValidatorID != validatorID {
				numFailed++
				logging.Logger.Error(
					"[challenge]resp: ",
					zap.String("validator",
						validatorID),
					zap.Any("resp", string(resp)),
					zap.Any("error", "Validator ID mismatch"),
				)
				updateMapAndSlice(validatorID, i, nil)
				return
			}

			logging.Logger.Info(
				"[challenge]resp: Got response from the validator.",
				zap.Any("validator_response", validationTicket),
				zap.Any("numSuccess", numSuccess),
				zap.Any("numFailed", numFailed),
			)

			verified, err := validationTicket.VerifySign()
			if err != nil || !verified {
				numFailed++
				logging.Logger.Error(
					"[challenge]ticket: Validation ticket from validator could not be verified.",
					zap.String("validator", validatorID),
				)
				updateMapAndSlice(validatorID, i, nil)
				return
			}
			updateMapAndSlice(validatorID, i, &validationTicket)

			if validationTicket.Result {
				numSuccess++
			} else {
				numFailed++
			}

		}(url, validator.ID, i)
	}

	for {
		if numSuccess > (len(cr.Validators)/2) || numSuccess+numFailed == len(cr.Validators) {
			break
		}
	}

	accessMu.RLock()
	logging.Logger.Info("[challenge]validator response stats", zap.Any("challenge_id", cr.ChallengeID), zap.Any("validator_responses", responses), zap.Any("num_success", numSuccess), zap.Any("num_failed", numFailed))
	accessMu.RUnlock()
	if numSuccess > (len(cr.Validators) / 2) {
		cr.Result = ChallengeSuccess
		cr.statusMutex.Lock()
		cr.Status = Processed
		cr.statusMutex.Unlock()
		cr.UpdatedAt = time.Now().UTC()
	} else {
		cr.CancelChallenge(ctx, ErrNoConsensusChallenge)
		return ErrNoConsensusChallenge
	}

	return nil
}

func IsValueNotPresentError(err error) bool {
	return strings.Contains(err.Error(), ValueNotPresent)
}

func IsEntityNotFoundError(err error) bool {
	return strings.Contains(err.Error(), EntityNotFound)
}

func (cr *ChallengeEntity) SaveChallengeResult(ctx context.Context, t *coreTxn.Transaction, toAdd bool) {
	cr.statusMutex.Lock()
	cr.Status = Committed
	cr.statusMutex.Unlock()
	cr.StatusMessage = t.TransactionOutput
	cr.CommitTxnID = t.Hash
	cr.UpdatedAt = time.Now().UTC()
	if toAdd {
		cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
	}
	if err := cr.Save(ctx); err != nil {
		logging.Logger.Error("[challenge]db: ", zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
	}
	if cr.RefID != 0 {
		FileChallenged(ctx, cr.RefID, cr.Result, cr.CommitTxnID)
	}

	txnVerification := time.Now()
	if err := UpdateChallengeTimingTxnVerification(cr.ChallengeID, common.Timestamp(txnVerification.Unix())); err != nil {
		logging.Logger.Error("[challengetiming]txnverification",
			zap.Any("challenge_id", cr.ChallengeID),
			zap.Time("created", common.ToTime(cr.CreatedAt)),
			zap.Time("txn_verified", txnVerification),
			zap.Error(err))
	}
}

func (cr *ChallengeEntity) getPostDataV2(ctx context.Context, allocationObj *allocation.Allocation) (map[string]any, error) {
	trie := allocationObj.GetTrie()
	copyTrie := wmpt.New(trie.CopyRoot(filestore.COLLAPSE_DEPTH), datastore.GetBlockStore())

	var (
		blockNum     = int64(0)
		postData     = make(map[string]interface{})
		ref          *reference.Ref
		objectSize   int64
		proofGenTime int64 = -1
	)
	if copyTrie.Weight() > 0 {
		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockNum = r.Int63n(int64(copyTrie.Weight()))
		blockNum++
		cr.BlockNum = blockNum
		logging.Logger.Info("[challenge]rand: ", zap.Uint64("trie.NumBlocks", trie.Weight()), zap.Any("blockNum", blockNum), zap.Any("challenge_id", cr.ChallengeID), zap.Any("random_seed", cr.RandomNumber))
		key, objectProof, err := copyTrie.GetBlockProof(uint64(blockNum))
		if err != nil {
			return nil, err
		}
		lookupHash := hex.EncodeToString(key)
		ref, err = reference.GetReferenceByLookupHash(ctx, cr.AllocationID, lookupHash)
		if err != nil {
			return nil, err
		}
		if ref.Type != reference.FILE {
			return nil, ErrInvalidObjectPath
		}
		cr.RefID = ref.ID
		postData["object_proof"] = objectProof
		postData["block_num"] = blockNum
		fileMetaRootBytes := copyTrie.Root()
		fileMetaRoot := hex.EncodeToString(fileMetaRootBytes)
		if fileMetaRoot != allocationObj.FileMetaRoot {
			logging.Logger.Error("root_mismatch", zap.Any("file_meta_root", allocationObj.FileMetaRoot), zap.Any("file_meta_root", fileMetaRoot))
			return nil, common.NewError("root_mismatch", "File meta root mismatch")
		}
	}
	cr.RespondedAllocationRoot = allocationObj.AllocationRoot
	if blockNum > 0 {
		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockoffset := r.Intn(sdkUtil.FixedMerkleLeaves)
		challengeReadInput := &filestore.ChallengeReadBlockInput{
			Hash:             ref.LookupHash,
			FileSize:         ref.Size,
			BlockOffset:      blockoffset,
			AllocationID:     cr.AllocationID,
			IsPrecommit:      ref.AllocationRoot == allocationObj.AllocationRoot,
			FilestoreVersion: ref.FilestoreVersion,
		}
		t1 := time.Now()
		challengeResponse, err := filestore.GetFileStore().GetBlocksMerkleTreeForChallenge(challengeReadInput)
		if err != nil {
			return nil, err
		}
		proofGenTime := time.Since(t1).Milliseconds()
		logging.Logger.Info("Proof gen logs: ",
			zap.Int64("block num", blockNum),
			zap.Int64("file size", ref.Size),
			zap.String("file path", ref.Path),
			zap.Int64("proof gen time", proofGenTime),
		)
		postData["challenge_proof"] = challengeResponse
		objectSize = ref.Size
		metaRef := &reference.RefMeta{
			Path:                    ref.Path,
			LookupHash:              ref.LookupHash,
			ActualFileSize:          ref.ActualFileSize,
			ActualFileHashSignature: ref.ActualFileHashSignature,
			ActualFileHash:          ref.ActualFileHash,
			ValidationRoot:          ref.ValidationRoot,
			ValidationRootSignature: ref.ValidationRootSignature,
			FixedMerkleRoot:         ref.FixedMerkleRoot,
			Size:                    ref.Size,
			FileMetaHash:            ref.FileMetaHash,
		}
		postData["meta"] = metaRef
	}
	err := UpdateChallengeTimingProofGenerationAndFileSize(
		cr.ChallengeID,
		proofGenTime,
		objectSize,
	)
	if err != nil {
		logging.Logger.Error("[challengetiming]txnverification",
			zap.Any("challenge_id", cr.ChallengeID),
			zap.Time("created", common.ToTime(cr.CreatedAt)),
			zap.Int64("proof_gen_time", int64(proofGenTime)),
			zap.Error(err))

		return nil, err
	}
	return postData, nil
}

func (cr *ChallengeEntity) getPostData(ctx context.Context, allocationObj *allocation.Allocation) (map[string]any, error) {
	rootRef, err := reference.GetReference(ctx, cr.AllocationID, "/")
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	blockNum := int64(0)
	var objectPath *reference.ObjectPath
	if rootRef != nil {
		if rootRef.Hash != allocationObj.AllocationRoot {
			logging.Logger.Error("root_mismatch", zap.Any("allocation_root", allocationObj.AllocationRoot), zap.Any("root_ref_hash", rootRef.Hash))
		}
		if rootRef.NumBlocks > 0 {
			r := rand.New(rand.NewSource(cr.RandomNumber))
			blockNum = r.Int63n(rootRef.NumBlocks)
			blockNum++
			cr.BlockNum = blockNum
		}

		logging.Logger.Info("[challenge]rand: ", zap.Any("rootRef.NumBlocks", rootRef.NumBlocks), zap.Any("blockNum", blockNum), zap.Any("challenge_id", cr.ChallengeID), zap.Any("random_seed", cr.RandomNumber))
		objectPath, err = reference.GetObjectPath(ctx, cr.AllocationID, blockNum)
		if err != nil {
			return nil, err
		}
		if objectPath != nil {
			cr.RefID = objectPath.RefID
			cr.ObjectPath = objectPath
		}
	}
	cr.RespondedAllocationRoot = allocationObj.AllocationRoot

	postData := make(map[string]interface{})
	if objectPath != nil {
		postData["object_path"] = objectPath
	}

	var proofGenTime int64 = -1

	if blockNum > 0 {
		if objectPath.Meta["type"] != reference.FILE {
			logging.Logger.Info("Block number to be challenged for file:", zap.Any("block", objectPath.FileBlockNum), zap.Any("meta", objectPath.Meta), zap.Any("obejct_path", objectPath))
			return nil, ErrInvalidObjectPath
		}

		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockoffset := r.Intn(sdkUtil.FixedMerkleLeaves)

		fromPreCommit := true

		if objectPath.Meta["is_precommit"] != nil {
			fromPreCommit = objectPath.Meta["is_precommit"].(bool)
			if fromPreCommit {
				fromPreCommit = objectPath.Meta["validation_root"].(string) != objectPath.Meta["prev_validation_root"].(string)
			}
		} else {
			logging.Logger.Error("is_precommit_is_nil", zap.Any("object_path", objectPath))
		}

		challengeReadInput := &filestore.ChallengeReadBlockInput{
			Hash:             objectPath.Meta["validation_root"].(string),
			FileSize:         objectPath.Meta["size"].(int64),
			BlockOffset:      blockoffset,
			AllocationID:     cr.AllocationID,
			IsPrecommit:      fromPreCommit,
			FilestoreVersion: objectPath.FilestoreVersion,
		}

		t1 := time.Now()
		challengeResponse, err := filestore.GetFileStore().GetBlocksMerkleTreeForChallenge(challengeReadInput)

		if err != nil {
			return nil, common.NewError("blockdata_not_found", err.Error())
		}
		proofGenTime = time.Since(t1).Milliseconds()

		if objectPath.Meta["size"] != nil {
			logging.Logger.Info("Proof gen logs: ",
				zap.Int64("block num", blockNum),
				zap.Int64("file size", objectPath.Meta["size"].(int64)),
				zap.String("file path", objectPath.Meta["name"].(string)),
				zap.Int64("proof gen time", proofGenTime),
			)
		}
		postData["challenge_proof"] = challengeResponse
	}

	if objectPath == nil {
		objectPath = &reference.ObjectPath{}
	}
	err = UpdateChallengeTimingProofGenerationAndFileSize(
		cr.ChallengeID,
		proofGenTime,
		objectPath.Size,
	)
	if err != nil {
		logging.Logger.Error("[challengetiming]txnverification",
			zap.Any("challenge_id", cr.ChallengeID),
			zap.Time("created", common.ToTime(cr.CreatedAt)),
			zap.Int64("proof_gen_time", int64(proofGenTime)),
			zap.Error(err))

		return nil, err
	}
	return postData, nil
}

func (cr *ChallengeEntity) VerifyChallengeTransaction(ctx context.Context, txn *coreTxn.Transaction) error {
	if len(cr.LastCommitTxnIDs) > 0 {
		for _, lastTxn := range cr.LastCommitTxnIDs {
			logging.Logger.Info("[challenge]commit: Verifying the transaction : " + lastTxn)
			t, err := coreTxn.VerifyTransaction(lastTxn)
			if err == nil && t.Status != coreTxn.TxnSuccess {
				cr.SaveChallengeResult(ctx, t, false)
				return nil
			}
			logging.Logger.Error("[challenge]trans: Error verifying the txn from BC."+lastTxn, zap.String("challenge_id", cr.ChallengeID), zap.Error(err))
		}
	}

	logging.Logger.Info("Verifying challenge response to blockchain.", zap.String("txn", txn.Hash), zap.String("challenge_id", cr.ChallengeID))
	var (
		t   *coreTxn.Transaction
		err error
	)
	for i := 0; i < 3; i++ {
		t, err = coreTxn.VerifyTransaction(txn.Hash)
		if err == nil {
			break
		}
		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	}

	if t == nil || err != nil || t.Status != coreTxn.TxnSuccess {
		logging.Logger.Error("Error verifying the challenge response transaction",
			zap.String("err:", err.Error()),
			zap.String("txn", txn.Hash),
			zap.String("challenge_id", cr.ChallengeID))

		if t != nil {
			cr.CommitTxnID = t.Hash
			cr.LastCommitTxnIDs = append(cr.LastCommitTxnIDs, t.Hash)
		}

		if IsEntityNotFoundError(err) {
			err = ErrEntityNotFound
		}
		_ = cr.Save(ctx)
		return err
	}
	logging.Logger.Info("Success response from BC for challenge response transaction", zap.String("txn", txn.TransactionOutput), zap.String("challenge_id", cr.ChallengeID))
	cr.SaveChallengeResult(ctx, t, true)
	return nil
}
