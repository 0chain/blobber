package node

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/0chain/common/core/logging"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	"github.com/0chain/gosdk/core/zcncrypto"
	"golang.org/x/crypto/sha3"
)

/*SelfNode -- self node type*/
type SelfNode struct {
	URL       string
	wallet    *zcncrypto.Wallet
	ID        string
	PublicKey string
}

/*SetKeys - setter */
func (sn *SelfNode) SetKeys(publicKey, privateKey string) {
	publicKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		panic(err)
	}
	sn.wallet = &zcncrypto.Wallet{}
	sn.wallet.ClientID = Hash(publicKeyBytes)
	sn.wallet.ClientKey = publicKey
	sn.wallet.Keys = make([]zcncrypto.KeyPair, 1)
	sn.wallet.Keys[0].PublicKey = publicKey
	sn.wallet.Keys[0].PrivateKey = privateKey
	sn.wallet.Version = zcncrypto.CryptoVersion

	sn.PublicKey = publicKey
	sn.ID = sn.wallet.ClientID
}

/*SetHostURL - setter for Host and Port */
func (sn *SelfNode) SetHostURL(schema, address string, port int) {
	if address == "" {
		address = "localhost"
	}

	if schema == "" {
		schema = "http"
	}

	sn.URL = fmt.Sprintf("%v://%v:%v", schema, address, port)
}

/*GetURLBase - get the end point base */
func (sn *SelfNode) GetURLBase() string {
	return sn.URL
}

/*Sign - sign the given hash */
func (sn *SelfNode) Sign(hash string) (string, error) {
	logging.Logger.Info("Sign hash", zap.String("hash", hash))
	logging.Logger.Info("Setting private key", zap.Any("sn.Wallet", sn))

	//return encryption.Sign(sn.privateKey, hash)
	signScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	if signScheme != nil {
		err := signScheme.SetPrivateKey(sn.wallet.Keys[0].PrivateKey)
		if err != nil {
			return "", err
		}
		logging.Logger.Info("Signing hash", zap.String("hash", hash))
		return signScheme.Sign(hash)
	}

	logging.Logger.Info("Invalid signature scheme", zap.String("scheme", config.Configuration.SignatureScheme))
	return "", common.NewError("invalid_signature_scheme", "Invalid signature scheme. Please check configuration")
}

func (sn *SelfNode) GetWallet() *zcncrypto.Wallet {
	return sn.wallet
}

func (sn *SelfNode) GetWalletString() string {
	walletStr, _ := json.Marshal(sn.wallet)
	return string(walletStr)
}

/*Self represents the node of this intance */
var Self SelfNode

const HASH_LENGTH = 32

type HashBytes [HASH_LENGTH]byte

/*Hash - hash the given data and return the hash as hex string */
func Hash(data interface{}) string {
	return hex.EncodeToString(RawHash(data))
}

/*RawHash - Logic to hash the text and return the hash bytes */
func RawHash(data interface{}) []byte {
	var databuf []byte
	switch dataImpl := data.(type) {
	case []byte:
		databuf = dataImpl
	case HashBytes:
		databuf = dataImpl[:]
	case string:
		databuf = []byte(dataImpl)
	default:
		panic("unknown type")
	}
	hash := sha3.New256()
	hash.Write(databuf)
	var buf []byte
	return hash.Sum(buf)
}
