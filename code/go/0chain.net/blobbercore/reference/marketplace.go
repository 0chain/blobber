package reference

import (
	"0chain.net/blobbercore/datastore"
	"context"
	"github.com/0chain/gosdk/core/zcncrypto"
	zboxenc "github.com/0chain/gosdk/zboxcore/encryption"
)

type MarketplaceInfo struct {
	Mnemonic   string    `gorm:"mnemonic" json:"mnemonic,omitempty"`
	PublicKey   string    `gorm:"public_key" json:"public_key"`
	PrivateKey  string    `gorm:"private_key" json:"private_key,omitempty"`
}

type KeyPairInfo struct {
	PublicKey string
	PrivateKey string
	Mnemonic string
}

func TableName() string {
	return "marketplace"
}

func AddEncryptionKeyPairInfo(ctx context.Context, keyPairInfo KeyPairInfo) error {
	db := datastore.GetStore().GetTransaction(ctx)
	return db.Table(TableName()).Create(&MarketplaceInfo{
		PrivateKey: keyPairInfo.PrivateKey,
		PublicKey: keyPairInfo.PublicKey,
		Mnemonic: keyPairInfo.Mnemonic,
	}).Error
}

func GetMarketplaceInfo(ctx context.Context) (MarketplaceInfo, error) {
	db := datastore.GetStore().GetTransaction(ctx)
	marketplaceInfo := MarketplaceInfo{}
	err := db.Table(TableName()).First(&marketplaceInfo).Error
	return marketplaceInfo, err
}

func GetSignatureScheme() zcncrypto.SignatureScheme {
	// TODO: bls0chain scheme crashes
	return zcncrypto.NewSignatureScheme("ed25519")
}

func GetMarketplaceInfoFromMnemonic(mnemonic string) *MarketplaceInfo {
	encscheme := zboxenc.NewEncryptionScheme()
	encscheme.Initialize(mnemonic)

	PrivateKey, _ := encscheme.GetPrivateKey()
	PublicKey, _ := encscheme.GetPublicKey()

	return &MarketplaceInfo{
		PrivateKey: PrivateKey,
		PublicKey: PublicKey,
		Mnemonic: mnemonic,
	}
}

func GetSecretKeyPair() (*KeyPairInfo, error) {
	wallet, err := zcncrypto.NewSignatureScheme("ed25519").GenerateKeys()
	if err != nil {
		return nil, err
	}
	return &KeyPairInfo {
		PublicKey: wallet.Keys[0].PublicKey,
		PrivateKey: wallet.Keys[0].PrivateKey,
		Mnemonic: wallet.Mnemonic,
	}, nil
}

func GetOrCreateMarketplaceInfo(ctx context.Context) (*MarketplaceInfo, error) {
	row, err := GetMarketplaceInfo(ctx)
	if err == nil {
		return &row, err
	}

	keyPairInfo, err := GetSecretKeyPair()

	if err != nil {
		return nil, err
	}

	AddEncryptionKeyPairInfo(ctx, *keyPairInfo)

	return &MarketplaceInfo{
		PrivateKey: keyPairInfo.PrivateKey,
		PublicKey: keyPairInfo.PublicKey,
		Mnemonic: keyPairInfo.Mnemonic,
	}, nil
}
