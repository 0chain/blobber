package mock

import (
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/0chain/gosdk/sdks"
	"github.com/0chain/gosdk/sdks/blobber"
)

const (
	zboxWallet = "{\"client_id\":\"9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85\",\"client_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"keys\":[{\"public_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"private_key\":\"a3a88aad5d89cec28c6e37c2925560ce160ac14d2cdcf4a4654b2bb358fe7514\"}],\"mnemonics\":\"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown\",\"version\":\"1.0\",\"date_created\":\"2021-05-21 17:32:29.484657 +0545 +0545 m=+0.072791323\"}"
)

const (
	mockOwnerWallet = "{\"client_id\":\"5d0229e0141071c1f88785b1faba4b612582f9d446b02e8d893f1e0d0ce92cdc\",\"client_key\":\"aefef5778906680360cf55bf462823367161520ad95ca183445a879a59c9bf0470b74e41fc12f2ee0ce9c19c4e77878d734226918672d089f561ecf1d5435720\",\"keys\":[{\"public_key\":\"aefef5778906680360cf55bf462823367161520ad95ca183445a879a59c9bf0470b74e41fc12f2ee0ce9c19c4e77878d734226918672d089f561ecf1d5435720\",\"private_key\":\"4f8af6fb1098a3817d705aef96db933f31755674b00a5d38bb2439c0a27b0117\"}],\"mnemonics\":\"erode transfer noble civil ridge cloth sentence gauge board wheel sight caution okay sand ranch ice frozen frown grape lion feed fox game zone\",\"version\":\"1.0\",\"date_created\":\"2021-09-04T14:11:06+01:00\"}"
)

func NewBlobberClient() *blobber.Blobber {

	z := sdks.New("9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85", "40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a", "bls0chain")
	z.InitWallet(zboxWallet)
	z.NewRequest = func(method, url string, body io.Reader) (*http.Request, error) {
		return httptest.NewRequest(method, url, body), nil
	}
	return blobber.New(z, "http://127.0.0.1:5051/")
}

func InitServer() {
	//client.PopulateClient(mockOwnerWallet, "bls0chain")
}
