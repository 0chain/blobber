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

func registerOnChain() error {
	//wait http & grpc startup, and go to setup on chain
	fmt.Print("> connecting to chain	\n")

	const ATTEMPT_DELAY = 60 * 1

	// setup wallet
	fmt.Print("	+ connect to miners: ")
	if isIntegrationTest {
		fmt.Print("	[SKIP]\n")
		return nil
	}

	if err := handler.WalletRegister(); err != nil {
		fmt.Println(err.Error() + "\n")
		panic(err)
	}
	fmt.Print("	[OK]\n")

	var err error
	// setup blobber (add or update) on the blockchain (multiple attempts)
	for i := 1; i <= ATTEMPT_DELAY; i++ {
		if i == 1 {
			fmt.Printf("\r	+ connect to sharders:")
		} else {
			time.Sleep(1 * time.Second)
			fmt.Printf("\r	+ [%v/10]connect to sharders:", i)
		}

		err = filestore.GetFileStore().CalculateCurrentDiskCapacity()
		if err != nil {
			continue
		}

		err = handler.RegisterBlobber(common.GetRootContext())
		if err != nil {
			continue
		}

		break
	}

	if !isIntegrationTest {
		go setupWorkers()

		go startHealthCheck()
		go startRefreshSettings()

		if config.Configuration.PriceInUSD {
			go refreshPriceOnChain()
		}
	}

	return err
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
