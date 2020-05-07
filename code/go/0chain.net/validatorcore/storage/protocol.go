package storage

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/core/chain"
	"0chain.net/core/common"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"
	"0chain.net/validatorcore/config"

	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
)

const CHUNK_SIZE = 64 * 1024
const ALLOCATION_CONTEXT_KEY common.ContextKey = "allocation"
const CLIENT_CONTEXT_KEY common.ContextKey = "client"
const CLIENT_KEY_CONTEXT_KEY common.ContextKey = "client_key"

type StorageNode struct {
	ID        string `json:"id"`
	BaseURL   string `json:"url"`
	PublicKey string `json:"-"`
}

//ValidatorProtocolImpl - implementation of the storage protocol
type ValidatorProtocolImpl struct {
	ServerChain *chain.Chain
}

func GetProtocolImpl() *ValidatorProtocolImpl {
	return &ValidatorProtocolImpl{
		ServerChain: chain.GetServerChain()}
}

// func (sp *ValidatorProtocolImpl) AddChallenge(ctx context.Context) (string, error) {
// 	txn := transaction.NewTransactionEntity()

// 	ri64, err := securerandom.Int64()
// 	if err != nil {
// 		return "", common.NewError("random_num_gen_failure", "Failed to generate a random number"+err.Error())
// 	}
// 	sn := make(map[string]int64)
// 	sn["random_number"] = ri64

// 	scData := &transaction.SmartContractTxnData{}
// 	scData.Name = sct.ADD_CHALLENGE_SC_NAME
// 	scData.InputArgs = sn

// 	txn.ToClientID = sct.STORAGE_CONTRACT_ADDRESS
// 	txn.Value = 0
// 	txn.TransactionType = transaction.TxnTypeSmartContract
// 	txnBytes, err := json.Marshal(scData)
// 	if err != nil {
// 		return "", err
// 	}
// 	txn.TransactionData = string(txnBytes)

// 	err = txn.ComputeHashAndSign()
// 	if err != nil {
// 		Logger.Info("Signing Failed during adding challenge", zap.String("err:", err.Error()))
// 		return "", err
// 	}
// 	transaction.SendTransaction(txn, sp.ServerChain)
// 	return txn.Hash, nil
// }

func (sp *ValidatorProtocolImpl) VerifyAllocationTransaction(ctx context.Context, allocationID string) (*Allocation, error) {
	t, err := transaction.VerifyTransaction(allocationID, sp.ServerChain)
	if err != nil {
		return nil, common.NewError("invalid_allocation", "Invalid Allocation id. Allocation not found in blockchain. "+err.Error())
	}
	var allocationObj Allocation
	err = json.Unmarshal([]byte(t.TransactionOutput), &allocationObj)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error", "Error decoding the allocation transaction output."+err.Error())
	}
	return &allocationObj, nil
}

func (sp *ValidatorProtocolImpl) VerifyChallengeTransaction(ctx context.Context, challengeRequest *ChallengeRequest) (*Challenge, error) {
	blobberID := ctx.Value(CLIENT_CONTEXT_KEY).(string)
	if len(blobberID) == 0 {
		return nil, common.NewError("invalid_client", "Call from an invalid client")
	}
	params := make(map[string]string)
	params["blobber"] = blobberID
	params["challenge"] = challengeRequest.ChallengeID
	challengeBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/getchallenge", params, chain.GetServerChain(), nil)

	if err != nil {
		return nil, common.NewError("invalid_challenge", "Invalid challenge id. Challenge not found in blockchain. "+err.Error())
	}
	var challengeObj Challenge
	err = json.Unmarshal(challengeBytes, &challengeObj)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error", "Error decoding the challenge output."+err.Error())
	}
	foundValidator := false
	for _, validator := range challengeObj.Validators {
		if validator.ID == node.Self.ID {
			foundValidator = true
			break
		}
	}
	if !foundValidator {
		return nil, common.NewError("invalid_challenge", "Validator is not part of the challenge")
	}

	if challengeObj.Blobber.ID != blobberID {
		return nil, common.NewError("invalid_challenge", "Challenge is meant for a different blobber")
	}

	return &challengeObj, nil
}

type WalletCallback struct {
	wg  *sync.WaitGroup
	err string
}

func (wb *WalletCallback) OnWalletCreateComplete(status int, wallet string, err string) {
	wb.err = err
	wb.wg.Done()
}

func (sp *ValidatorProtocolImpl) RegisterValidator(ctx context.Context) (string, error) {

	wcb := &WalletCallback{}
	wcb.wg = &sync.WaitGroup{}
	wcb.wg.Add(1)
	err := zcncore.RegisterToMiners(node.Self.GetWallet(), wcb)
	if err != nil {
		return "", err
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return "", err
	}

	sn := &transaction.StorageNode{}
	sn.ID = node.Self.GetKey()
	sn.BaseURL = node.Self.GetURLBase()
	sn.DelegateWallets = config.Configuration.DelegateWallets

	snBytes, err := json.Marshal(sn)
	if err != nil {
		return "", err
	}
	Logger.Info("Adding validator to the blockchain.")
	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.ADD_VALIDATOR_SC_NAME, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed during registering validator to the mining network", zap.String("err:", err.Error()))
		return "", err
	}

	return txn.Hash, nil
}
