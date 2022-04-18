package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
)

func setupOnChain(step int) {
	//wait http & grpc startup, and go to setup on chain
	time.Sleep(1 * time.Second)
	fmt.Printf("[%v/%v] connecting to chain	\n", step, totalSteps)

	const ATTEMPT_DELAY = 60 * 1

	// setup wallet
	fmt.Print("	+ connect to miners: ")
	if isIntegrationTest {
		fmt.Print("	[SKIP]\n")
	} else {
		if err := handler.WalletRegister(); err != nil {
			fmt.Println(err.Error() + "\n")
			panic(err)
		}
		fmt.Print("	[OK]\n")
	}

	// setup blobber (add or update) on the blockchain (multiple attempts)
	for i := 1; i <= 10; i++ {
		if i == 1 {
			fmt.Printf("\r	+ connect to sharders:")
		} else {
			fmt.Printf("\r	+ [%v/10]connect to sharders:", i)
		}

		if isIntegrationTest {
			fmt.Print("	[SKIP]\n")
			break
		} else {
			if err := registerBlobberOnChain(); err != nil {
				if i == 10 { // no more attempts
					panic(err)
				}
				fmt.Print("\n		", err.Error()+"\n")
			} else {
				fmt.Print("	[OK]\n")
				break
			}
			for n := 0; n < ATTEMPT_DELAY; n++ {
				<-time.After(1 * time.Second)

				fmt.Printf("\r	- wait %v seconds to retry", ATTEMPT_DELAY-n)
			}
		}
	}
	if !isIntegrationTest {
		go setupWorkers()

		go healthCheckOnChain()

		if config.Configuration.PriceInUSD {
			go refreshPriceOnChain()
		}
	}
}

func registerBlobberOnChain() error {
	txnHash, err := handler.BlobberAdd(common.GetRootContext())
	if err != nil {
		return err
	}

	if t, err := handler.TransactionVerify(txnHash); err != nil {
		logging.Logger.Error("Failed to verify blobber add/update transaction", zap.Any("err", err), zap.String("txn.Hash", txnHash))
	} else {
		logging.Logger.Info("Verified blobber add/update transaction", zap.String("txn_hash", t.Hash), zap.Any("txn_output", t.TransactionOutput))
	}

	return err
}

func setupServerChain(step int) error {
	fmt.Printf("[%v/%v] setup server chain", step, totalSteps)
	common.SetupRootContext(node.GetNodeContext())

	config.SetServerChainID(config.Configuration.ChainID)
	serverChain := chain.NewChainFromConfig()
	chain.SetServerChain(serverChain)

	if err := zcncore.InitZCNSDK(serverChain.BlockWorker, config.Configuration.SignatureScheme); err != nil {
		if isIntegrationTest {
			return nil
		}

		return err
	}
	if err := zcncore.SetWalletInfo(node.Self.GetWalletString(), false); err != nil {
		if isIntegrationTest {
			return nil
		}
		return err
	}

	fmt.Print("	[OK]\n")
	return nil
}
