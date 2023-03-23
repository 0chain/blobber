package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
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
)

type ChallengeResponse struct {
	nextCResp, prevCResp *ChallengeResponse // doubly linked list
	ChallengeCreatedAt   common.Timestamp
	notifyCh             chan NotifyStatus
	ChallengeID          string              `json:"challenge_id"`
	ValidationTickets    []*ValidationTicket `json:"validation_tickets"`
	Sequence             uint64
	ErrCh                chan error
	DoneCh               chan struct{}
	restartWithNonceCh   chan int64
	challengeTiming      *ChallengeTiming
}

func (cr *ChallengeEntity) SubmitChallengeToBC(ctx context.Context) (*transaction.Transaction, error) {
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return nil, err
	}

	sn := &ChallengeResponse{}
	sn.ChallengeID = cr.ChallengeID
	for _, vt := range cr.ValidationTickets {
		if vt != nil {
			sn.ValidationTickets = append(sn.ValidationTickets, vt)
		}
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.CHALLENGE_RESPONSE, sn, 0)
	if err != nil {
		logging.Logger.Info("Failed submitting challenge to the mining network", zap.String("err:", err.Error()))
		return nil, err
	}

	cr.ChallengeTiming.TxnSubmission = common.Now()

	logging.Logger.Info("Verifying challenge response to blockchain.", zap.String("txn", txn.Hash), zap.String("challenge_id", cr.ChallengeID))
	var (
		t *transaction.Transaction
	)

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err = transaction.VerifyTransactionWithNonce(txn.Hash, txn.GetTransaction().GetTransactionNonce())

	if err != nil {
		logging.Logger.Error("Error verifying the challenge response transaction",
			zap.String("err:", err.Error()),
			zap.String("txn", txn.Hash),
			zap.String("challenge_id", cr.ChallengeID))
		return txn, err
	}

	logging.Logger.Info("Challenge committed and accepted",
		zap.Any("txn.hash", t.Hash),
		zap.Any("txn.output", t.TransactionOutput),
		zap.String("challenge_id", cr.ChallengeID))

	return t, nil
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
	if len(cr.Validators) == 0 {

		cr.CancelChallenge(ErrNoValidator)
		return ErrNoValidator
	}

	// Lock allocation changes from happening in handler.CommitWrite function
	// This lock should be unlocked as soon as possible. We should not defer
	// unlocking it as it will be locked for longer time and handler.CommitWrite
	// will fail.
	allocMu := lock.GetMutex(allocation.Allocation{}.TableName(), cr.AllocationID)
	allocMu.Lock()

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

		cr.Status = Processed
		cr.UpdatedAt = time.Now().UTC()
	} else {
		cr.CancelChallenge(ErrNoConsensusChallenge)
		return ErrNoConsensusChallenge
	}

	return cr.Save(ctx)
}

var processChalCh = make(chan *ChallengeResponse)

func (cr *ChallengeEntity) CommitChallenge(ctx context.Context) error {
	cResp := &ChallengeResponse{
		ChallengeCreatedAt: cr.CreatedAt,
		ChallengeID:        cr.ChallengeID,
		ErrCh:              make(chan error, 1),
		restartWithNonceCh: make(chan int64, 1),
		challengeTiming:    cr.ChallengeTiming,
	}

	for _, vt := range cr.ValidationTickets {
		if vt != nil {
			cResp.ValidationTickets = append(cResp.ValidationTickets, vt)
		}
	}

	processChalCh <- cResp
	return <-cResp.ErrCh
}

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
			crp.nextCResp.prevCResp = nil
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
			for {
				if oldestCresp.prevCResp != nil {
					oldestCresp = oldestCresp.prevCResp
				} else {
					break
				}
			}

			for {
				status := <-oldestCresp.notifyCh
				var shouldRemove bool
				if status == Failed {
					shouldRemove = true
					oldestCresp.challengeTiming.ClosedAt = common.Now()
					goto L1
				} else if time.Since(common.ToTime(cResp.ChallengeCreatedAt)) > config.StorageSCConfig.ChallengeCompletionTime {
					shouldRemove = true
					oldestCresp.challengeTiming.ClosedAt = common.Now()
					oldestCresp.ErrCh <- ErrExpiredCCT
					oldestCresp.restartWithNonceCh <- 0 // send zero to cancel the awaiting goroutine
					goto L1
				}

				oldestCresp.restartWithNonceCh <- transaction.GetNextUnusedNonce()

			L1:
				if shouldRemove {
					if oldestCresp.prevCResp != nil {
						oldestCresp.prevCResp.nextCResp = oldestCresp.nextCResp
					}
					if oldestCresp.nextCResp != nil {
						oldestCresp.nextCResp.prevCResp = oldestCresp.prevCResp
					}
				}

				if oldestCresp.nextCResp != nil {
					oldestCresp = oldestCresp.nextCResp
				} else {
					break
				}
			}

			// deplete all the challenges in buffer
		D:
			for {
				select {
				case <-stopToProcessCh:
				default:
					break D
				}
			}

			latestCResp = oldestCresp
			dblLinkedMu.Unlock()
		}
	}()

	for cResp := range processChalCh {
		dblLinkedMu.Lock()
		guideCh <- struct{}{}

		cResp.prevCResp = latestCResp
		if latestCResp != nil {
			latestCResp.nextCResp = cResp
		}
		go tryChallengeTransaction(cResp, doneCh, stopToProcessCh, dblLinkedMu, guideCh, transaction.GetNextUnusedNonce())

		dblLinkedMu.Unlock()
	}
}

func IsValueNotPresentError(err error) bool {
	return strings.Contains(err.Error(), ValueNotPresent)
}

func tryChallengeTransaction(
	cResp *ChallengeResponse,
	doneCh chan *ChallengeResponse,
	stopToProcessCh chan *ChallengeResponse,
	dblLinkedMu *sync.Mutex,
	guideCh chan struct{},
	nonce int64,
) {
	defer func() {
		<-guideCh
	}()

	var txn *transaction.Transaction
	var err error

	ctx, ctxCncl := context.WithCancel(context.TODO())
	defer ctxCncl()

	var t *transaction.Transaction
	for {
		cResp.challengeTiming.RetriesInChain++
		for i := 0; i < 2; i++ {
			txn, err = transaction.NewTransactionEntity()
			if err != nil {
				goto L1
			}
			cResp.challengeTiming.TxnSubmission = common.Now()
			err = txn.ExecuteSmartContractWithNonce(
				transaction.STORAGE_CONTRACT_ADDRESS,
				transaction.CHALLENGE_RESPONSE,
				cResp,
				0,
				nonce,
			)
			if err == nil {
				break
			}
			logging.Logger.Info("Failed submitting challenge to the mining network", zap.String("err:", err.Error()))
		}
		if err != nil {
			goto L1
		}

		time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
		t, err = transaction.VerifyTransactionWithNonce(txn.Hash, txn.GetTransaction().GetTransactionNonce())

		if err != nil {
			logging.Logger.Error("Error verifying the challenge response transaction",
				zap.String("err:", err.Error()),
				zap.String("txn", txn.Hash),
				zap.String("challenge_id", cResp.ChallengeID))
			goto L1
		} else {
			logging.Logger.Info("Challenge committed and accepted",
				zap.Any("txn.hash", t.Hash),
				zap.Any("txn.output", t.TransactionOutput),
				zap.String("challenge_id", cResp.ChallengeID))

			cResp.challengeTiming.TxnVerification = common.Now()
			ctxCncl()
		}

	L1:
		select {
		case <-ctx.Done():
			doneCh <- cResp
			cResp.notifyCh <- Success
			cResp.ErrCh <- nil
			return
		default:
			if dblLinkedMu.TryLock() {
				select {
				case nonce = <-cResp.restartWithNonceCh:
					dblLinkedMu.Unlock()
					continue
				default:
					stopToProcessCh <- cResp
					cResp.notifyCh <- Failed
					cResp.ErrCh <- err
					dblLinkedMu.Unlock()
					return
				}
			}

			cResp.notifyCh <- Waiting
			nonce = <-cResp.restartWithNonceCh
			if nonce == 0 {
				return
			}
		}
	}
}
