package main

import (
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	handleCommon "github.com/0chain/blobber/code/go/0chain.net/core/common/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/0chain/gosdk/zcncore"
	"go.uber.org/zap"
	"time"
)

func registerOnChain() error {
	//wait http & grpc startup, and go to setup on chain
	fmt.Print("> connecting to chain	\n")

	const ATTEMPT_DELAY = 30 //30s

	// setup wallet
	fmt.Print("	+ connect to miners: ")

	fmt.Print("	[OK]\n")

	err := filestore.GetFileStore().CalculateCurrentDiskCapacity()
	if err != nil {
		return err
	}

	// setup blobber (add) on the blockchain (multiple attempts)
	i := 1
	for {
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
		if err == nil {
			break
		} else {
			i++
			logging.Logger.Error("add_blobber_error", zap.Error(err))
		}
	}

	if err != nil {
		return err
	}

	fmt.Print("	[OK]\n")

	ctx := common.GetRootContext()
	go setupWorkers(ctx)

	// go StartHealthCheck(ctx, common.ProviderTypeBlobber)
	go handleCommon.StartHealthCheck(ctx, common.ProviderTypeBlobber)
	go startRefreshSettings(ctx)

	return err
}

func setupServerChain() error {
	fmt.Print("> setup server chain")
	common.SetupRootContext(node.GetNodeContext())

	config.SetServerChainID(config.Configuration.ChainID)
	serverChain := chain.NewChainFromConfig()
	chain.SetServerChain(serverChain)

	if err := zcncore.InitZCNSDK(serverChain.BlockWorker, config.Configuration.SignatureScheme); err != nil {
		return err
	}
	if err := zcncore.SetWalletInfo(node.Self.GetWalletString(), false); err != nil {
		return err
	}

	if err := sdk.InitStorageSDK(node.Self.GetWalletString(), serverChain.BlockWorker, config.Configuration.ChainID, config.Configuration.SignatureScheme,
		nil, 0); err != nil {
		return err
	}

	fmt.Print("	[OK]\n")
	return nil
}
