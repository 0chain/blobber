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

	const ATTEMPT_DELAY = 30 //30s

	// setup wallet
	fmt.Print("	+ connect to miners: ")
	if isIntegrationTest {
		fmt.Print("	[SKIP]\n")
		return nil
	}

	var err error

	err = handler.WalletRegister()
	if err != nil {
		return err
	}
	fmt.Print("	[OK]\n")

	err = filestore.GetFileStore().CalculateCurrentDiskCapacity()
	if err != nil {
		return err
	}

	// setup blobber (add or update) on the blockchain (multiple attempts)
	for i := 1; i <= 10; i++ {
		if i == 1 {
			fmt.Printf("\r	+ connect to sharders:")
		} else {
			for n := ATTEMPT_DELAY; n > 0; n-- {
				if n == 1 {
					fmt.Printf("\r	+ [%v/10]connect to sharders:      ", i)
				} else {
					fmt.Printf("\r	+ [%v/10]connect to sharders: %.2vs", i, n)
				}

				<-time.After(1 * time.Second)
			}
		}

		err = handler.RegisterBlobber(common.GetRootContext())
		if err != nil {
			continue
		}

		break
	}

	if err != nil {
		return err
	}

	fmt.Print("	[OK]\n")

	if !isIntegrationTest {
		ctx := common.GetRootContext()
		go setupWorkers(ctx)

		go startHealthCheck(ctx)
		go startRefreshSettings(ctx)

		if config.Configuration.PriceInUSD {
			go refreshPriceOnChain(ctx)
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
