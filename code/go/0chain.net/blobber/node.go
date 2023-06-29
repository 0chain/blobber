package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"go.uber.org/zap"
)

func setupNode() error {
	fmt.Print("> setup blobber")

	reader, err := os.Open(keysFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	publicKey, privateKey, _, _ := encryption.ReadKeys(reader)

	node.Self.SetKeys(publicKey, privateKey)
	if node.Self.ID == "" {
		return errors.New("node definition for self node doesn't exist")
	} else {
		logging.Logger.Info("self identity", zap.Any("id", node.Self.ID))
	}

	if len(hostUrl) > 0 {
		node.Self.URL = hostUrl
	} else {
		if httpsPort > 0 {
			node.Self.SetHostURL("https", hostname, httpsPort)
		} else {
			node.Self.SetHostURL("http", hostname, httpPort)
		}
	}

	fmt.Println("*== Validator Wallet Info ==*")
	fmt.Println("	ID: ", node.Self.ID)
	fmt.Println("	Public Key: ", publicKey)
	fmt.Println("	Private Key: ", privateKey)
	fmt.Println("*===========================*")

	logging.Logger.Info(" Base URL" + node.Self.GetURLBase())
	fmt.Print("		[OK]\n")
	return nil
}
