package node

import (
	"encoding/json"
	"fmt"

	"0chain.net/core/common"
	"0chain.net/core/config"
	"0chain.net/core/encryption"
	"github.com/0chain/gosdk/core/zcncrypto"
)

/*SelfNode -- self node type*/
type SelfNode struct {
	*Node
	wallet     *zcncrypto.Wallet
	privateKey string
}

/*SetKeys - setter */
func (sn *SelfNode) SetKeys(publicKey string, privateKey string) {
	sn.privateKey = privateKey
	sn.SetPublicKey(publicKey)
	sn.wallet = &zcncrypto.Wallet{}
	sn.wallet.ClientID = sn.ID
	sn.wallet.ClientKey = sn.PublicKey
	sn.wallet.Keys = make([]zcncrypto.KeyPair, 1)
	sn.wallet.Keys[0].PublicKey = sn.PublicKey
	sn.wallet.Keys[0].PrivateKey = privateKey
	sn.wallet.Version = zcncrypto.CryptoVersion
}

/*SetHostURL - setter for Host and Port */
func (sn *SelfNode) SetHostURL(address string, port int) {
	sn.SetURLBase(address, port)
}

/*Sign - sign the given hash */
func (sn *SelfNode) Sign(hash string) (string, error) {
	//return encryption.Sign(sn.privateKey, hash)
	signScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	if signScheme != nil {
		err := signScheme.SetPrivateKey(sn.privateKey)
		if err != nil {
			return "", err
		}
		return signScheme.Sign(hash)
	}
	return "", common.NewError("invalid_signature_scheme", "Invalid signature scheme. Please check configuration")
}

/*TimeStampSignature - get timestamp based signature */
func (sn *SelfNode) TimeStampSignature() (string, string, string, error) {
	data := fmt.Sprintf("%v:%v", sn.ID, common.Now())
	hash := encryption.Hash(data)
	signature, err := sn.Sign(hash)
	if err != nil {
		return "", "", "", err
	}
	return data, hash, signature, err
}

func (sn *SelfNode) GetWallet() *zcncrypto.Wallet {
	return sn.wallet
}

func (sn *SelfNode) GetWalletString() string {
	walletStr, _ := json.Marshal(sn.wallet)
	return string(walletStr)
}

/*Self represents the node of this intance */
var Self *SelfNode

func init() {
	Self = &SelfNode{}
	Self.Node = &Node{}
}
