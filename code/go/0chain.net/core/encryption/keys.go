package encryption

import (
	"bufio"
	"io"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/config"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/herumi/bls-go-binary/bls"
)

func init() {
	err := bls.Init(bls.CurveFp254BNb)
	if err != nil {
		panic(err)
	}
}

/*
ReadKeys - reads a publicKey and a privateKey from a Reader.
They are assumed to be in two separate lines one followed by the other
*/
func ReadKeys(reader io.Reader) (publicKey, privateKey, publicIp, port string) {
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	publicKey = scanner.Text()
	scanner.Scan()
	privateKey = scanner.Text()
	scanner.Scan()

	publicIp = scanner.Text()
	scanner.Scan()
	port = scanner.Text()
	return publicKey, privateKey, publicIp, port
}

// Verify - given a public key and a signature and the hash used to create the signature, verify the signature
func Verify(publicKey, signature, hash string) (bool, error) {
	publicKey = MiraclToHerumiPK(publicKey)
	signature = MiraclToHerumiSig(signature)
	signScheme := zcncrypto.NewSignatureScheme(config.Configuration.SignatureScheme)
	if signScheme != nil {
		err := signScheme.SetPublicKey(publicKey)
		if err != nil {
			return false, err
		}
		return signScheme.Verify(signature, hash)
	}
	return false, common.NewError("invalid_signature_scheme", "Invalid signature scheme. Please check configuration")
}

// If input is normal herumi/bls public key, it returns it immmediately.
//
//	So this is completely backward compatible with herumi/bls.
//
// If input is MIRACL public key, convert it to herumi/bls public key.
//
// This is an example of the raw public key we expect from MIRACL
var miraclExamplePK = `0418a02c6bd223ae0dfda1d2f9a3c81726ab436ce5e9d17c531ff0a385a13a0b491bdfed3a85690775ee35c61678957aaba7b1a1899438829f1dc94248d87ed36817f6dfafec19bfa87bf791a4d694f43fec227ae6f5a867490e30328cac05eaff039ac7dfc3364e851ebd2631ea6f1685609fc66d50223cc696cb59ff2fee47ac`

// This is an example of the same MIRACL public key serialized with ToString().
// pk ([1bdfed3a85690775ee35c61678957aaba7b1a1899438829f1dc94248d87ed368,18a02c6bd223ae0dfda1d2f9a3c81726ab436ce5e9d17c531ff0a385a13a0b49],[039ac7dfc3364e851ebd2631ea6f1685609fc66d50223cc696cb59ff2fee47ac,17f6dfafec19bfa87bf791a4d694f43fec227ae6f5a867490e30328cac05eaff])
func MiraclToHerumiPK(pk string) string {
	if len(pk) != len(miraclExamplePK) {
		// If input is normal herumi/bls public key, it returns it immmediately.
		return pk
	}
	n1 := pk[2:66]
	n2 := pk[66:(66 + 64)]
	n3 := pk[(66 + 64):(66 + 64 + 64)]
	n4 := pk[(66 + 64 + 64):(66 + 64 + 64 + 64)]
	var p bls.PublicKey
	err := p.SetHexString("1 " + n2 + " " + n1 + " " + n4 + " " + n3)
	if err != nil {
		Logger.Error("MiraclToHerumiPK: " + err.Error())
	}
	return p.SerializeToHexStr()
}

// Converts signature 'sig' to format that the herumi/bls library likes.
// zwallets are using MIRACL library which send a MIRACL signature not herumi
// lib.
//
// If the 'sig' was not in MIRACL format, we just return the original sig.
const miraclExampleSig = `(0d4dbad6d2586d5e01b6b7fbad77e4adfa81212c52b4a0b885e19c58e0944764,110061aa16d5ba36eef0ad4503be346908d3513c0a2aedfd0d2923411b420eca)`

func MiraclToHerumiSig(sig string) string {
	if len(sig) <= 2 {
		return sig
	}
	if sig[0] != miraclExampleSig[0] {
		return sig
	}
	withoutParens := sig[1:(len(sig) - 1)]
	comma := strings.Index(withoutParens, ",")
	if comma < 0 {
		return "00"
	}
	n1 := withoutParens[0:comma]
	n2 := withoutParens[(comma + 1):]
	var sign bls.Sign
	err := sign.SetHexString("1 " + n1 + " " + n2)
	if err != nil {
		Logger.Error("MiraclToHerumiSig: " + err.Error())
	}
	return sign.SerializeToHexStr()
}

func AggregateSig(signatures []string) (string, error) {
	var sig bls.Sign
	sigVec := make([]bls.Sign, len(signatures))
	for i := 0; i < len(signatures); i++ {
		err := sigVec[i].DeserializeHexStr(signatures[i])
		if err != nil {
			Logger.Error("AggregateSig: " + err.Error())
			return "", err
		}
	}
	sig.Aggregate(sigVec)
	return sig.SerializeToHexStr(), nil
}
