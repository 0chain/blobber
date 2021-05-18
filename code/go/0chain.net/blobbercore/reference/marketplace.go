package reference

import (
	"context"
	"0chain.net/core/config"
	"0chain.net/blobbercore/datastore"
	"github.com/0chain/gosdk/core/zcncrypto"
	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

type MarketplaceInfo struct {
	PublicKey   string    `gorm:"public_key" json:"public_key"`
	PrivateKey  string    `gorm:"private_key" json:"private_key,omitempty"`
}

func TableName() string {
	return "marketplace"
}

func AddEncryptionKeyPair(ctx context.Context, privateKey string, publicKey string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Table(TableName()).Create(&MarketplaceInfo{
		PrivateKey: privateKey,
		PublicKey: publicKey,
	}).Error
}

func GetMarketplaceInfo(ctx context.Context) (MarketplaceInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	marketplaceInfo := MarketplaceInfo{}
	err := db.Table(TableName()).First(&marketplaceInfo).Error
	return marketplaceInfo, err
}

func GetSecretKeyPair() (*zcncrypto.KeyPair, error) {
	//sigScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	// TODO: bls0chain scheme crashes
	sigScheme := zcncrypto.NewSignatureScheme("ed25519")
	wallet, err := sigScheme.GenerateKeys()
	if err != nil {
		return nil, err
	}
	return &wallet.Keys[0], nil
}

func GetOrCreateMarketplaceInfo(ctx context.Context) (*MarketplaceInfo, error) {
	row, err := GetMarketplaceInfo(ctx)
	if err == nil {
		return &row, err
	}

	Logger.Info("Creating key pair", zap.String("signature_scheme", config.Configuration.SignatureScheme))
	keyPair, err := GetSecretKeyPair()
	Logger.Info("Secret key pair created")

	if err != nil {
		return nil, err
	}

	AddEncryptionKeyPair(ctx, keyPair.PrivateKey, keyPair.PublicKey)

	return &MarketplaceInfo{
		PublicKey: keyPair.PublicKey,
		PrivateKey: keyPair.PrivateKey,
	}, nil
}
