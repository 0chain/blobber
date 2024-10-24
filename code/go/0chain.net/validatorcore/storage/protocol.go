package storage

import (
	"context"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/gosdk/constants"
	"sync"
)

const CHUNK_SIZE = 64 * 1024

type StorageNode struct {
	ID        string `json:"id"`
	BaseURL   string `json:"url"`
	PublicKey string `json:"-"`
}

// ValidatorProtocolImpl - implementation of the storage protocol
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

func (sp *ValidatorProtocolImpl) VerifyChallengeTransaction(ctx context.Context, challengeRequest *ChallengeRequest) (*Challenge, error) {
	blobberID := ctx.Value(constants.ContextKeyClient).(string)
	if blobberID == "" {
		return nil, common.NewError("invalid_client", "Call from an invalid client")
	}
	params := make(map[string]string)
	params["blobber"] = blobberID
	params["challenge"] = challengeRequest.ChallengeID
	challengeBytes, err := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/getchallenge", params)

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

	if challengeObj.BlobberID != blobberID {
		return nil, common.NewError("invalid_challenge", "Challenge is meant for a different blobber")
	}

	return &challengeObj, nil
}

type WalletCallback struct {
	wg  *sync.WaitGroup
	err string
}

func (wb *WalletCallback) OnWalletCreateComplete(status int, wallet, err string) {
	wb.err = err
	wb.wg.Done()
}
