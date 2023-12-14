package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"go.uber.org/zap"
)

var publicKey, privateKey string

func setupNode() error {
	fmt.Print("> setup blobber")

	err := readKeysFromAws()
	if err != nil {
		err = readKeysFromFile(&keysFile)
		if err != nil {
			panic(err)
		}
		fmt.Print("using blobber keys from local")
	} else {
		fmt.Print("using blobber keys from aws")
	}

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

	fmt.Println("*== Blobber Wallet Info ==*")
	fmt.Println("	ID: ", node.Self.ID)
	fmt.Println("	Public Key: ", publicKey)
	fmt.Println("*===========================*")

	logging.Logger.Info(" Base URL" + node.Self.GetURLBase())
	fmt.Print("		[OK]\n")
	return nil
}

func readKeysFromAws() error {
	blobberSecretName := os.Getenv("BLOBBER_SECRET_NAME")
	awsRegion := os.Getenv("AWS_REGION")
	keys, err := common.GetSecretsFromAWS(blobberSecretName, awsRegion)
	if err != nil {
		return err
	}
	secretsFromAws := strings.Split(keys, "\n")
	if len(secretsFromAws) < 2 {
		return fmt.Errorf("wrong file format from aws")
	}
	publicKey = secretsFromAws[0]
	privateKey = secretsFromAws[1]
	return nil
}

func readKeysFromFile(keysFile *string) error {
	reader, err := os.Open(*keysFile)
	if err != nil {
		return err
	}
	defer reader.Close()
	publicKey, privateKey, _, _ = encryption.ReadKeys(reader)
	return nil
}