package encryption

import (
	"github.com/0chain/gosdk/zboxcore/client"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSignatureVerify(t *testing.T) {
	allocationId := "4f928c7857fabb5737347c42204eea919a4777f893f35724f563b932f64e2367"
	walletConfig := "{\"client_id\":\"9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85\",\"client_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"keys\":[{\"public_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"private_key\":\"a3a88aad5d89cec28c6e37c2925560ce160ac14d2cdcf4a4654b2bb358fe7514\"}],\"mnemonics\":\"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown\",\"version\":\"1.0\",\"date_created\":\"2021-05-21 17:32:29.484657 +0545 +0545 m=+0.072791323\"}"
	client.PopulateClient(walletConfig, "bls0chain")
	sig, serr := client.Sign(allocationId)
	assert.Nil(t, serr)
	assert.NotNil(t, sig)

	res, err := client.VerifySignature(
		"fb0eb9351978091da350348211888b06ed1ce84ae40d08de3cc826cd85197188",
		allocationId,
	)
	assert.Nil(t, err)
	assert.Equal(t, res, true)
}