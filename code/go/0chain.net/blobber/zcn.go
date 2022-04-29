package main

import (
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zcncore"
)

func setupOnChain() {
	//wait http & grpc startup, and go to setup on chain
	time.Sleep(1 * time.Second)
	fmt.Print("> connecting to chain	\n")

	const ATTEMPT_DELAY = 60 * 1

	// setup wallet
	fmt.Print("	+ connect to miners: ")
	if isIntegrationTest {
		fmt.Print("	[SKIP]\n")
		return
	}

	if err := handler.WalletRegister(); err != nil {
		fmt.Println(err.Error() + "\n")
		panic(err)
	}
	fmt.Print("	[OK]\n")

	var success bool
	var err error
	// setup blobber (add or update) on the blockchain (multiple attempts)
	for i := 1; i <= 10; i++ {
		fmt.Printf("\r	+ [%v/10]connect to sharders:", i)
		if err = filestore.GetFileStore().CalculateCurrentDiskCapacity(); err != nil {
			fmt.Print("\n		", err.Error()+"\n")
			goto sleep
		}

		if err = handler.RegisterBlobber(common.GetRootContext()); err != nil {
			fmt.Print("\n		", err.Error()+"\n")
			goto sleep
		}

		fmt.Print("	[OK]\n")
		success = true
		break

	sleep:
		for n := 0; n < ATTEMPT_DELAY; n++ {
			<-time.After(1 * time.Second)

			fmt.Printf("\r	- wait %v seconds to retry", ATTEMPT_DELAY-n)
		}
	}

	if !success {
		panic(err)
	}

	if !isIntegrationTest {
		go setupWorkers()

		go startHealthCheck()
		go startRefreshSettings()

		if config.Configuration.PriceInUSD {
			go refreshPriceOnChain()
		}
	}
}

func setupServerChain() error {
	fmt.Print("> setup server chain")
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
