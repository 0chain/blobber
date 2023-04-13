package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/blobber/code/go/0chain.net/core/util"
	sdkUtil "github.com/0chain/gosdk/core/util"
	"github.com/remeh/sizedwaitgroup"

	"go.uber.org/zap"
)

const VALIDATOR_URL = "/v1/storage/challenge/new"
const ValueNotPresent = "value not present"

var (
	ErrNoValidator          = errors.New("no validators assigned to the challenge")
	ErrNoConsensusChallenge = errors.New("no_consensus_challenge: No Consensus on the challenge result. Erroring out the challenge")
	ErrInvalidObjectPath    = errors.New("invalid_object_path: Object path was not for a file")
	ErrExpiredCCT           = errors.New("expired challenge completion time")
	ErrValNotPresent        = errors.New("chain responded: " + ValueNotPresent)
	ErrFailedChallenge      = errors.New("failed to execute challenge response transaction")
)

type ChallengeResponse struct {
	nextCResp, prevCResp *ChallengeResponse  `json:"-"` // doubly linked list
	ChallengeCreatedAt   common.Timestamp    `json:"-"`
	notifyCh             chan NotifyStatus   `json:"-"`
	ChallengeID          string              `json:"challenge_id"`
	ValidationTickets    []*ValidationTicket `json:"validation_tickets"`
	Sequence             uint64              `json:"-"`
	ErrCh                chan error          `json:"-"`
	DoneCh               chan struct{}       `json:"-"`
	restartWithNonceCh   chan int64          `json:"-"`
	challengeTiming      *ChallengeTiming    `json:"-"`
}

func (cr *ChallengeEntity) CancelChallenge(errReason error) {
	cancellation := common.Now()
	cr.ChallengeTiming.ClosedAt = cancellation
	if errReason == ErrExpiredCCT {
		cr.ChallengeTiming.Expiration = cancellation
	}
}

// LoadValidationTickets load validation tickets
func (cr *ChallengeEntity) LoadValidationTickets(ctx context.Context) error {
	logging.Logger.Info("Loading validation tickets")
	if len(cr.Validators) == 0 {
		logging.Logger.Error("No validators")
		cr.CancelChallenge(ErrNoValidator)
		return ErrNoValidator
	}

	logging.Logger.Info("Acquiring lock")
	// Lock allocation changes from happening in handler.CommitWrite function
	// This lock should be unlocked as soon as possible. We should not defer
	// unlocking it as it will be locked for longer time and handler.CommitWrite
	// will fail.
	allocMu := lock.GetMutex(allocation.Allocation{}.TableName(), cr.AllocationID)
	allocMu.Lock()
	logging.Logger.Info("Lock acquired")

	allocationObj, err := allocation.GetAllocationByID(ctx, cr.AllocationID)
	if err != nil {
		allocMu.Unlock()
		cr.CancelChallenge(ErrNoValidator)
		return err
	}

	wms, err := writemarker.GetWriteMarkersInRange(ctx, cr.AllocationID, cr.AllocationRoot, allocationObj.AllocationRoot)
	if err != nil {
		allocMu.Unlock()
		return err
	}
	if len(wms) == 0 {
		allocMu.Unlock()
		return common.NewError("write_marker_not_found", "Could find the writemarker for the given allocation root on challenge")
	}

	rootRef, err := reference.GetReference(ctx, cr.AllocationID, "/")
	if err != nil {
		allocMu.Unlock()
		cr.CancelChallenge(err)
		return err
	}

	blockNum := int64(0)
	if rootRef.NumBlocks > 0 {
		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockNum = r.Int63n(rootRef.NumBlocks)
		blockNum++
		cr.BlockNum = blockNum
	}

	logging.Logger.Info("[challenge]rand: ", zap.Any("rootRef.NumBlocks", rootRef.NumBlocks), zap.Any("blockNum", blockNum), zap.Any("challenge_id", cr.ChallengeID), zap.Any("random_seed", cr.RandomNumber))
	objectPath, err := reference.GetObjectPath(ctx, cr.AllocationID, blockNum)
	if err != nil {
		allocMu.Unlock()
		cr.CancelChallenge(err)
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

	var proofGenTime int64 = -1

	if blockNum > 0 {
		if objectPath.Meta["type"] != reference.FILE {
			allocMu.Unlock()
			logging.Logger.Info("Block number to be challenged for file:", zap.Any("block", objectPath.FileBlockNum), zap.Any("meta", objectPath.Meta), zap.Any("obejct_path", objectPath))

			cr.CancelChallenge(ErrInvalidObjectPath)
			return ErrInvalidObjectPath
		}

		r := rand.New(rand.NewSource(cr.RandomNumber))
		blockoffset := r.Intn(sdkUtil.FixedMerkleLeaves)

		challengeReadInput := &filestore.ChallengeReadBlockInput{
			Hash:         objectPath.Meta["validation_root"].(string),
			FileSize:     objectPath.Meta["size"].(int64),
			BlockOffset:  blockoffset,
			AllocationID: cr.AllocationID,
		}

		t1 := time.Now()
		challengeResponse, err := filestore.GetFileStore().GetBlocksMerkleTreeForChallenge(challengeReadInput)

		if err != nil {
			allocMu.Unlock()
			cr.CancelChallenge(err)
			return common.NewError("blockdata_not_found", err.Error())
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

	logging.Logger.Info("Proof gen logs: ",
		zap.Int64("block num", blockNum),
		zap.Int64("file size", objectPath.Meta["size"].(int64)),
		zap.String("file path", objectPath.Meta["name"].(string)),
		zap.Int64("proof gen time", proofGenTime),
	)

	cr.ChallengeTiming.FileSize = objectPath.Size
	cr.ChallengeTiming.ProofGenTime = proofGenTime

	allocMu.Unlock()

	postDataBytes, err := json.Marshal(postData)
	if err != nil {
		logging.Logger.Error("[db]form: " + err.Error())
		cr.CancelChallenge(err)
		return err
	}
	responses := make(map[string]ValidationTicket)
	if cr.ValidationTickets == nil {
		cr.ValidationTickets = make([]*ValidationTicket, len(cr.Validators))
	}

	accessMu := sync.Mutex{}
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
				//network issue, don't cancel it, and try it again
				logging.Logger.Info("[challenge]post: ", zap.Any("error", err.Error()))
				updateMapAndSlice(validatorID, i, nil)
				return
			}
			var validationTicket ValidationTicket
			err = json.Unmarshal(resp, &validationTicket)
			if err != nil {
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
			logging.Logger.Info(
				"[challenge]resp: Got response from the validator.",
				zap.Any("validator_response", validationTicket),
			)
			verified, err := validationTicket.VerifySign()
			if err != nil || !verified {
				logging.Logger.Error(
					"[challenge]ticket: Validation ticket from validator could not be verified.",
					zap.String("validator", validatorID),
				)
				updateMapAndSlice(validatorID, i, nil)
				return
			}
			updateMapAndSlice(validatorID, i, &validationTicket)
		}(url, validator.ID, i)
	}

	swg.Wait()

	numSuccess := 0
	numFailure := 0

	numValidatorsResponded := 0
	for _, vt := range cr.ValidationTickets {
		if vt != nil {
			if vt.Result {
				numSuccess++
			} else {
				logging.Logger.Error("[challenge]ticket: "+vt.Message, zap.String("validator", vt.ValidatorID))
				numFailure++
			}
			numValidatorsResponded++
		}
	}

	logging.Logger.Info("[challenge]validator response stats", zap.Any("challenge_id", cr.ChallengeID), zap.Any("validator_responses", responses))
	if numSuccess > (len(cr.Validators)/2) || numFailure > (len(cr.Validators)/2) || numValidatorsResponded == len(cr.Validators) {
		if numSuccess > (len(cr.Validators) / 2) {
			cr.Result = ChallengeSuccess
		} else {
			cr.Result = ChallengeFailure

			logging.Logger.Error("[challenge]validate: ", zap.String("challenge_id", cr.ChallengeID), zap.Any("block_num", cr.BlockNum), zap.Any("object_path", objectPath))
		}

		cr.UpdatedAt = time.Now().UTC()
	} else {
		cr.CancelChallenge(ErrNoConsensusChallenge)
		return ErrNoConsensusChallenge
	}

	return nil
}

var commitChalCh = make(chan *ChallengeResponse)

type NotifyStatus int

const (
	Success NotifyStatus = iota
	Failed
	Waiting
)

// TODO Add sequence number in each challenge and compare sequence number to put them in order
// Trying to solve following problem.
// Consider challenges C1, C2 and C3 with transactions T1, T2 and T3 with nonce 1, 2 and 3 respectively.
// Lets suppose that we send these transactions asynchronously to the blockchain and C2 failed immediately and is not going to succeed.
// C1 is still processing.
// C3 is not going to work because its previous nonce is missing.
// First we should check if C1 is still processing and if so wait for it.
// Otherwise if we send C3 with nonce 2 and then C1 fails then again C3 will fail.
// Wait for C1. If comes with error then remove it and send C3 with nonce 1.
//
// We need to also take care of the fact that there are other processes(write/read marker redeeming) that uses
// nonceMonitor which will update latest nonce value.

func ProcessChallengeTransactions(ctx context.Context) {
	dblLinkedMu := &sync.Mutex{}
	var latestCResp *ChallengeResponse
	const guideNum = 10
	guideCh := make(chan struct{}, guideNum)
	stopToProcessCh := make(chan *ChallengeResponse, 1)
	doneCh := make(chan *ChallengeResponse, 1) // if done unlink from next cResp

	go func() {
		for crp := range doneCh {
			select {
			case <-ctx.Done():
				return
			default:
			}

			dblLinkedMu.Lock()
			crp.challengeTiming.ClosedAt = common.Now()
			if crp.nextCResp != nil {
				crp.nextCResp.prevCResp = nil
			}
			if crp.prevCResp != nil {
				crp.prevCResp.nextCResp = crp.nextCResp
			}
			dblLinkedMu.Unlock()
		}
	}()

	go func() {
		for cResp := range stopToProcessCh {
			select {
			case <-ctx.Done():
				return
			default:
			}

			dblLinkedMu.Lock()
			oldestCresp := cResp
			for oldestCresp.prevCResp != nil {
				oldestCresp = oldestCresp.prevCResp
			}

			for {
				status := <-oldestCresp.notifyCh
				if status == Failed || time.Since(common.ToTime(cResp.ChallengeCreatedAt)) > config.StorageSCConfig.ChallengeCompletionTime {
					oldestCresp.challengeTiming.ClosedAt = common.Now()
					if status == Failed {
						oldestCresp.ErrCh <- ErrFailedChallenge
					} else {
						oldestCresp.ErrCh <- ErrExpiredCCT
					}
					oldestCresp.restartWithNonceCh <- 0 // send zero to cancel the awaiting goroutine
					if oldestCresp.prevCResp != nil {
						oldestCresp.prevCResp.nextCResp = oldestCresp.nextCResp
					}
					if oldestCresp.nextCResp != nil {
						oldestCresp.nextCResp.prevCResp = oldestCresp.prevCResp
					}
				} else {
					oldestCresp.restartWithNonceCh <- transaction.GetNextUnusedNonce()
				}

				if oldestCresp.nextCResp != nil {
					oldestCresp = oldestCresp.nextCResp
				} else {
					break
				}
			}

			depleteChallengeBuffer(stopToProcessCh)

			latestCResp = oldestCresp
			dblLinkedMu.Unlock()
		}
	}()

	for cResp := range commitChalCh {
		guideCh <- struct{}{}
		dblLinkedMu.Lock()
		logging.Logger.Info("Committing challenge", zap.String("challenge_id", cResp.ChallengeID))
		cResp.prevCResp = latestCResp
		if latestCResp != nil {
			latestCResp.nextCResp = cResp
		}
		latestCResp = cResp
		params := challengeTransactionParams{
			cResp:           cResp,
			doneCh:          doneCh,
			stopToProcessCh: stopToProcessCh,
			dblLinkedMu:     dblLinkedMu,
			guideCh:         guideCh,
			nonce:           transaction.GetNextUnusedNonce(),
		}
		go tryChallengeTransaction(params)

		dblLinkedMu.Unlock()
	}
}

type challengeTransactionParams struct {
	cResp           *ChallengeResponse
	doneCh          chan *ChallengeResponse
	stopToProcessCh chan *ChallengeResponse
	dblLinkedMu     *sync.Mutex
	guideCh         chan struct{}
	nonce           int64
}

func tryChallengeTransaction(params challengeTransactionParams) {
	defer func() {
		<-params.guideCh
	}()

	var txn *transaction.Transaction
	var err error

	ctx, ctxCncl := context.WithCancel(context.Background())
	defer ctxCncl()

	for {
		params.cResp.challengeTiming.RetriesInChain++
		for i := 0; i < 2; i++ {
			txn, err = transaction.NewTransactionEntity()
			if err != nil {
				break
			}
			params.cResp.challengeTiming.TxnSubmission = common.Now()
			err = txn.ExecuteSmartContractWithNonce(
				transaction.STORAGE_CONTRACT_ADDRESS,
				transaction.CHALLENGE_RESPONSE,
				params.cResp,
				0,
				params.nonce,
			)
			if err == nil {
				break
			}
			logging.Logger.Info("Failed submitting challenge to the mining network", zap.Error(err))
		}
		if err != nil {
			break
		}

		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		t, err := transaction.VerifyTransactionWithNonce(txn.Hash, txn.GetTransaction().GetTransactionNonce())

		if err != nil {
			logging.Logger.Error("Error verifying the challenge response transaction",
				zap.Error(err),
				zap.String("txn", txn.Hash),
				zap.String("challenge_id", params.cResp.ChallengeID))
			break
		} else {
			logging.Logger.Info("Challenge committed and accepted",
				zap.Any("txn.hash", t.Hash),
				zap.Any("txn.output", t.TransactionOutput),
				zap.String("challenge_id", params.cResp.ChallengeID))

			params.cResp.challengeTiming.TxnVerification = common.Now()
			ctxCncl()
		}
	}

	select {
	case <-ctx.Done():
		params.dblLinkedMu.Lock()
		defer params.dblLinkedMu.Unlock()
		params.doneCh <- params.cResp
		params.cResp.notifyCh <- Success
		params.cResp.ErrCh <- nil
	default:
		params.dblLinkedMu.Lock()
		defer params.dblLinkedMu.Unlock()
		if params.nonce, err = handleChallengeTransactionError(params.cResp, params.stopToProcessCh, params.nonce); err != nil {
			params.cResp.notifyCh <- Failed
			params.cResp.ErrCh <- err
			return
		}
		params.cResp.notifyCh <- Waiting
		params.nonce = <-params.cResp.restartWithNonceCh
		if params.nonce == 0 {
			return
		}
	}
}

func handleChallengeTransactionError(cResp *ChallengeResponse, stopToProcessCh chan *ChallengeResponse, nonce int64) (int64, error) {
	oldestCresp := cResp
	for oldestCresp.prevCResp != nil {
		oldestCresp = oldestCresp.prevCResp
	}

	for {
		status := <-oldestCresp.notifyCh
		if status == Failed || time.Since(common.ToTime(cResp.ChallengeCreatedAt)) > config.StorageSCConfig.ChallengeCompletionTime {
			oldestCresp.challengeTiming.ClosedAt = common.Now()
			if status == Failed {
				return nonce, ErrFailedChallenge
			}
			return nonce, ErrExpiredCCT
		}

		nonce = transaction.GetNextUnusedNonce()
		oldestCresp.restartWithNonceCh <- nonce

		if oldestCresp.nextCResp != nil {
			oldestCresp = oldestCresp.nextCResp
		} else {
			break
		}
	}

	depleteChallengeBuffer(stopToProcessCh)
	return nonce, nil
}

func depleteChallengeBuffer(stopToProcessCh chan *ChallengeResponse) {
	for {
		select {
		case <-stopToProcessCh:
		default:
			return
		}
	}
}

// seqManagerCh will simply receive challenges in order and based on the status
// received on the statusCh it will send challenge to commit to blockchain.
var seqManagerCh = make(chan *ChallengeEntity, 10)

func sequenceManager(ctx context.Context) {
	cct := config.StorageSCConfig.ChallengeCompletionTime

	for chalEnt := range seqManagerCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		d := cct - time.Since(common.ToTime(chalEnt.CreatedAt))
		t := time.NewTimer(d)
		var status ChallengeStatus
		select {
		case status = <-chalEnt.StatusCh:
		case <-t.C:
			continue
		}

		if status == Completed {
			cResp := &ChallengeResponse{
				ChallengeCreatedAt: chalEnt.CreatedAt,
				ChallengeID:        chalEnt.ChallengeID,
				notifyCh:           make(chan NotifyStatus, 1),
				ErrCh:              chalEnt.ErrCh,
				restartWithNonceCh: make(chan int64, 1),
				challengeTiming:    chalEnt.ChallengeTiming,
			}

			for _, vt := range chalEnt.ValidationTickets {
				if vt != nil {
					cResp.ValidationTickets = append(cResp.ValidationTickets, vt)
				}
			}
			commitChalCh <- cResp
		}
	}
}
