package encryption

import (
	"testing"

	"github.com/0chain/gosdk/core/client"
	"github.com/0chain/gosdk/core/sys"
	"github.com/herumi/bls-go-binary/bls"
	"github.com/stretchr/testify/require"
)

func TestSignatureVerify(t *testing.T) {
	allocationId := "4f928c7857fabb5737347c42204eea919a4777f893f35724f563b932f64e2367"
	walletConfig := "{\"client_id\":\"9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85\",\"client_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"keys\":[{\"public_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"private_key\":\"a3a88aad5d89cec28c6e37c2925560ce160ac14d2cdcf4a4654b2bb358fe7514\"}],\"mnemonics\":\"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown\",\"version\":\"1.0\",\"date_created\":\"2021-05-21 17:32:29.484657 +0545 +0545 m=+0.072791323\"}"
	_, err := client.PopulateClient(walletConfig, "bls0chain")
	require.NoError(t, err)
	sig, serr := client.Sign(allocationId)
	require.Nil(t, serr)
	require.NotNil(t, sig)

	res, err := sys.Verify(
		"fb0eb9351978091da350348211888b06ed1ce84ae40d08de3cc826cd85197188",
		allocationId,
	)
	require.Nil(t, err)
	require.Equal(t, res, true)
}

func TestMiraclToHerumiPK(t *testing.T) {
	miraclpk1 := `0418a02c6bd223ae0dfda1d2f9a3c81726ab436ce5e9d17c531ff0a385a13a0b491bdfed3a85690775ee35c61678957aaba7b1a1899438829f1dc94248d87ed36817f6dfafec19bfa87bf791a4d694f43fec227ae6f5a867490e30328cac05eaff039ac7dfc3364e851ebd2631ea6f1685609fc66d50223cc696cb59ff2fee47ac`
	pk1 := MiraclToHerumiPK(miraclpk1)

	require.EqualValues(t, pk1, "68d37ed84842c91d9f82389489a1b1a7ab7a957816c635ee750769853aeddf1b490b3aa185a3f01f537cd1e9e56c43ab2617c8a3f9d2a1fd0dae23d26b2ca018")

	// Assert DeserializeHexStr works on the output of MiraclToHerumiPK
	var pk bls.PublicKey
	err := pk.DeserializeHexStr(pk1)
	require.NoError(t, err)
}

func TestMiraclToHerumiSig(t *testing.T) {
	miraclsig1 := `(0d4dbad6d2586d5e01b6b7fbad77e4adfa81212c52b4a0b885e19c58e0944764,110061aa16d5ba36eef0ad4503be346908d3513c0a2aedfd0d2923411b420eca)`
	sig1 := MiraclToHerumiSig(miraclsig1)

	require.EqualValues(t, sig1, "644794e0589ce185b8a0b4522c2181faade477adfbb7b6015e6d58d2d6ba4d0d")

	// Assert DeserializeHexStr works on the output of MiraclToHerumiSig
	var sig bls.Sign
	err := sig.DeserializeHexStr(sig1)
	require.NoError(t, err)

	// Test that passing in normal herumi sig just gets back the original.
	sig2 := MiraclToHerumiSig(sig1)
	if sig1 != sig2 {
		panic("Signatures should be the same.")
	}
}

// Helper code to print out expected values of Hash and conversion functions.
func TestDebugOnly(t *testing.T) {

	// clientKey := "536d2ecfe5aab6c343e8c2e7ee9daa60c43eecc53f4b1c07a6cb2648d9e66c14f2e3fcd43875be40722992f56570fe3c751caacbc7d859b309c787f654bd5a97"
	// // => 5c2fdfa03fc013cff0e4b716f0529b914e18fd2bc6cdfed49df13b6e3dc4684d

	//clientKey := "0416c528570ce46eb83584cd604a9ed62644ef4f71a86587d57e4ab91953ff4699107374870799ad4550c4f3833cca2a4d5de75436d67caf89097f1e7d6d7de6d424cb5a08b9dca8957ea7c81a23d066b93a27500954cd29733149ec1f8a8abd540d08f9f81bb24b83ff27e24f173e639573e10a22ed7b0ca326a1aa9dc03e1eef"
	// => bd3adcacc78ed4352931b138729986a07d2bf0e0a3bf2c885b37a9a0e649dd87
	// Looking for bd3adcacc78ed4352931b138729986a07d2bf0e0a3bf2c885b37a9a0e649dd87

	//clientKeyBytes, _ := hex.DecodeString(clientKey)
	//	h := Hash(clientKeyBytes)

	//fmt.Println("hash ", h)

	//herumipk := MiraclToHerumiPK(clientKey)
	//fmt.Println("herumipk ", herumipk)
	//clientKeyBytes2, _ := hex.DecodeString(herumipk)
	//	h = Hash(clientKeyBytes2)
	//fmt.Println("hash2 ", h)

}
