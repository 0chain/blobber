package handler

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"

	"github.com/0chain/gosdk/zboxcore/client"
	mocket "github.com/selvatico/go-mocket"

	"github.com/0chain/blobber/code/go/0chain.net/core/node"

	"github.com/0chain/gosdk/zboxcore/fileref"

	"github.com/0chain/gosdk/zboxcore/blockchain"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	c_common "github.com/0chain/gosdk/core/common"
	"github.com/0chain/gosdk/zboxcore/marker"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"testing"
)

func TestDownloadFile(t *testing.T) {
	const (
		mocketLogging    = true
		mockBlobberId    = "mock_blobber_id"
		mockAllocationId = "mock_allocation_id"
		mockAllocationTx = "mock_allocation_Tx"
		//mockClientId       = "9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85"
		mockRemoteFilePath = "mock/remote/file/path"
		mockBlockNumber    = 1
		mockEncryptKey     = "mock encrypt key"
		//mockPublicKey      = "40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a"
		//mockPrivateKey     = "��6\u0000O�u\v|(�iy�A��}�c��ĳ��\u0006���9\u0014"
		mockClientWallet = "{\"client_id\":\"9a566aa4f8e8c342fed97c8928040a21f21b8f574e5782c28568635ba9c75a85\",\"client_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"keys\":[{\"public_key\":\"40cd10039913ceabacf05a7c60e1ad69bb2964987bc50f77495e514dc451f907c3d8ebcdab20eedde9c8f39b9a1d66609a637352f318552fb69d4b3672516d1a\",\"private_key\":\"a3a88aad5d89cec28c6e37c2925560ce160ac14d2cdcf4a4654b2bb358fe7514\"}],\"mnemonics\":\"inside february piece turkey offer merry select combine tissue wave wet shift room afraid december gown mean brick speak grant gain become toy clown\",\"version\":\"1.0\",\"date_created\":\"2021-05-21 17:32:29.484657 +0545 +0545 m=+0.072791323\"}"
		mockOwnerWallet  = "{\"client_id\":\"5d0229e0141071c1f88785b1faba4b612582f9d446b02e8d893f1e0d0ce92cdc\",\"client_key\":\"aefef5778906680360cf55bf462823367161520ad95ca183445a879a59c9bf0470b74e41fc12f2ee0ce9c19c4e77878d734226918672d089f561ecf1d5435720\",\"keys\":[{\"public_key\":\"aefef5778906680360cf55bf462823367161520ad95ca183445a879a59c9bf0470b74e41fc12f2ee0ce9c19c4e77878d734226918672d089f561ecf1d5435720\",\"private_key\":\"4f8af6fb1098a3817d705aef96db933f31755674b00a5d38bb2439c0a27b0117\"}],\"mnemonics\":\"erode transfer noble civil ridge cloth sentence gauge board wheel sight caution okay sand ranch ice frozen frown grape lion feed fox game zone\",\"version\":\"1.0\",\"date_created\":\"2021-09-04T14:11:06+01:00\"}"
		//mockRemoteFilePathHash = "mock remote file path hash"
		mockReadPrice  = int64(0.1 * 1e10)
		mockWritePrice = int64(0.5 * 1e10)
		mockBigBalance = int64(10000 * 1e10)
		mockPoolId     = "mock pool id"
	)
	ts := time.Now().Add(time.Hour)
	var mockLongTimeInFuture = common.Timestamp(ts.Unix()) + common.Timestamp(time.Second*1000)
	var mockClient client.Client
	require.NoError(t, json.Unmarshal([]byte(mockClientWallet), &mockClient))
	var mockOwner client.Client
	require.NoError(t, json.Unmarshal([]byte(mockOwnerWallet), &mockOwner))
	var (
		now = c_common.Timestamp(time.Now().Unix())
	)

	type (
		blockDownloadRequest struct {
			blobber        *blockchain.StorageNode
			allocationID   string
			allocationTx   string
			remotefilepath string
			pathHash       string
			blockNum       int64
			encryptedKey   string
			contentMode    string
			numBlocks      int64
			rxPay          bool
		}

		parameters struct {
			isOwner         bool
			isCollaborator  bool
			useAuthTicket   bool
			attribute       common.WhoPays
			payerId         client.Client
			isRevoked       bool
			isFundedBlobber bool
			isFunded0Chain  bool
			rxPay           bool

			// client input from gosdk's BlockDownloadRequest,
			inData blockDownloadRequest

			// input from blobber database
			allocation allocation.Allocation
		}
		args struct {
			request *http.Request
		}
		want struct {
			blobberIds []int
			err        bool
			errMsg     string
		}
		test struct {
			name       string
			parameters parameters
			mockSetup  func(sqlmock.Sqlmock)
			want       want
		}
	)
	node.Self.ID = mockBlobberId

	var getPayerId = func(p parameters) client.Client {
		if p.isOwner || p.isCollaborator || p.attribute == common.WhoPaysOwner {
			return mockOwner
		}
		return mockClient
	}

	var addToForm = func(
		t *testing.T,
		formWriter *multipart.Writer,
		p parameters,
	) *marker.ReadMarker {
		rm := &marker.ReadMarker{}
		rm.ClientID = client.GetClientID()
		rm.ClientPublicKey = client.GetClientPublicKey()
		rm.BlobberID = p.inData.blobber.ID
		rm.AllocationID = p.inData.allocationID
		rm.OwnerID = mockOwner.ClientID
		rm.Timestamp = now
		rm.ReadCounter = p.inData.numBlocks
		err := rm.Sign()
		require.NoError(t, err)
		rmData, err := json.Marshal(rm)
		require.NoError(t, err)
		//pathHash := fileref.GetReferenceLookup(p.inData.allocationID, p.inData.remotefilepath)
		formWriter.WriteField("path_hash", p.inData.pathHash)
		formWriter.WriteField("path", p.inData.remotefilepath)
		if p.inData.rxPay {
			formWriter.WriteField("rx_pay", "true") // pay oneself
		}
		formWriter.WriteField("block_num", fmt.Sprintf("%d", p.inData.blockNum))
		formWriter.WriteField("num_blocks", fmt.Sprintf("%d", p.inData.numBlocks))
		formWriter.WriteField("read_marker", string(rmData))
		if p.useAuthTicket {
			authTicket := &marker.AuthTicket{
				AllocationID: p.inData.allocationID,
				ClientID:     client.GetClientID(),
				Expiration:   int64(time.Duration(now) + 10000*time.Second),
				OwnerID:      mockOwner.ClientID,
				Timestamp:    int64(common.Now()),
				FilePathHash: p.inData.pathHash,
			}
			require.NoError(t, client.PopulateClient(mockOwnerWallet, "bls0chain"))
			authTicket.Sign()
			require.NoError(t, client.PopulateClient(mockClientWallet, "bls0chain"))
			authTicketBytes, _ := json.Marshal(authTicket)
			formWriter.WriteField("auth_token", string(authTicketBytes))
		}
		if len(p.inData.contentMode) > 0 {
			formWriter.WriteField("content", p.inData.contentMode)
		}
		formWriter.Close()
		return rm
	}

	makeMockMakeSCRestAPICall := func(t *testing.T, p parameters) func(scAddress string, relativePath string, params map[string]string, chain *chain.Chain) ([]byte, error) {
		return func(scAddress string, relativePath string, params map[string]string, chain *chain.Chain) ([]byte, error) {
			require.New(t)
			require.EqualValues(t, scAddress, transaction.STORAGE_CONTRACT_ADDRESS)
			switch relativePath {
			case "/getReadPoolAllocBlobberStat":
				require.False(t, p.isFundedBlobber)
				for key, value := range params {
					switch key {
					case "client_id":
						require.EqualValues(t, p.payerId.ClientID, value)
					case "allocation_id":
						require.EqualValues(t, mockAllocationId, value)
					case "blobber_id":
						require.EqualValues(t, mockBlobberId, value)
					default:
						require.Fail(t, "unexpected parameter "+key+" call "+relativePath)
					}
				}
				var pss []*allocation.PoolStat
				var funds int64
				if p.isFunded0Chain {
					funds = mockBigBalance
				}
				pss = append(pss, &allocation.PoolStat{
					PoolID:   mockPoolId,
					Balance:  funds,
					ExpireAt: mockLongTimeInFuture,
				})
				bytes, err := json.Marshal(&pss)
				require.NoError(t, err)
				return bytes, nil
			default:
				require.Fail(t, "unexpected REST API endpoint call: "+relativePath)
			}
			return []byte{}, nil
		}
	}

	setupInMock := func(
		t *testing.T,
		p parameters,
		rm marker.ReadMarker,
	) {
		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT * FROM "allocations" WHERE`,
		).WithArgs(
			"mock_allocation_Tx",
		).OneTime().WithReply(
			[]map[string]interface{}{{
				"id":               p.allocation.ID,
				"expiration_date":  mockLongTimeInFuture,
				"owner_id":         mockOwner.ClientID,
				"owner_public_key": mockOwner.ClientKey,
			}},
		)

		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT * FROM "terms" WHERE`,
		).WithArgs(
			"mock_allocation_id",
		).OneTime().WithReply(
			[]map[string]interface{}{{
				"blobber_id":  mockBlobberId,
				"read_price":  mockReadPrice,
				"write_price": mockWritePrice,
			}},
		)

		attribute, err := json.Marshal(&reference.Attributes{
			WhoPaysForReads: p.attribute,
		})
		require.NoError(t, err)
		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT * FROM "reference_objects" WHERE`,
		).WithArgs(
			"mock_allocation_id", p.inData.pathHash,
		).WithReply(
			[]map[string]interface{}{{
				"allocation_id": p.allocation.ID,
				"lookup_hash":   p.inData.pathHash,
				"type":          reference.FILE,
				"attributes":    attribute,
			}},
		)

		var collaboratorRtv int
		if p.isCollaborator {
			collaboratorRtv = 1
		}
		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT count(1) FROM "collaborators" WHERE`,
		).WithArgs(
			mockClient.ClientID,
		).WithCallback(func(par1 string, args []driver.NamedValue) {
			fmt.Println(args)
		}).WithReply(
			[]map[string]interface{}{{"count": collaboratorRtv}},
		)

		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT * FROM "read_markers" WHERE`,
		).WithCallback(func(par1 string, args []driver.NamedValue) {
			require.EqualValues(t, p.payerId.ClientID, args[0].Value)
		}).WithArgs(
			p.payerId.ClientID,
		).WithReply(
			[]map[string]interface{}{{
				"client_id":       p.allocation.ID,
				"redeem_required": false,
			}},
		)

		mocket.Catcher.NewMock().OneTime().WithQuery(
			`SELECT * FROM "read_markers" WHERE`,
		).WithCallback(func(par1 string, args []driver.NamedValue) {
			//require.EqualValues(t, p.payerId.ClientID, args[0].Value)
			require.EqualValues(t, client.GetClientID(), args[0].Value)
		}).WithReply(
			[]map[string]interface{}{{
				"client_id":       p.allocation.ID,
				"redeem_required": false,
			}},
		)

		var funds int64
		if p.isFundedBlobber || p.isFunded0Chain {
			funds = mockBigBalance
		}

		fundedPool := []map[string]interface{}{{
			"pool_id":       "",
			"client_id":     p.payerId.ClientID,
			"blobber_id":    mockBlobberId,
			"allocation_id": mockAllocationId,
			"balance":       funds,
			"expire_at":     mockLongTimeInFuture,
		}}
		if p.isFundedBlobber {
			mocket.Catcher.NewMock().WithCallback(func(par1 string, args []driver.NamedValue) {
				require.EqualValues(t, p.payerId.ClientID, args[0].Value)
				require.EqualValues(t, mockAllocationId, args[1].Value)
				require.EqualValues(t, mockBlobberId, args[2].Value)
			}).OneTime().WithQuery(`SELECT * FROM "read_pools" WHERE`).WithReply(
				fundedPool,
			)
		} else {
			mocket.Catcher.NewMock().OneTime().WithQuery(
				`SELECT * FROM "read_pools" WHERE`,
			).WithCallback(func(_ string, args []driver.NamedValue) {
				require.EqualValues(t, p.payerId.ClientID, args[0].Value)
				require.EqualValues(t, mockAllocationId, args[1].Value)
				require.EqualValues(t, mockBlobberId, args[2].Value)
			}).WithReply(
				[]map[string]interface{}{},
			)

			mocket.Catcher.NewMock().OneTime().WithQuery(
				`SELECT * FROM "read_pools" WHERE`,
			).WithCallback(func(_ string, args []driver.NamedValue) {
				require.EqualValues(t, p.payerId.ClientID, args[0].Value)
				require.EqualValues(t, mockAllocationId, args[1].Value)
				require.EqualValues(t, mockBlobberId, args[2].Value)
			}).WithReply(
				fundedPool,
			)
		}

		if p.useAuthTicket {
			mocket.Catcher.NewMock().OneTime().WithQuery(
				`SELECT * FROM "reference_objects" WHERE`,
			).WithCallback(func(_ string, args []driver.NamedValue) {
				require.EqualValues(t, p.payerId.ClientID, args[0].Value)
				require.EqualValues(t, mockAllocationId, args[1].Value)
				require.EqualValues(t, mockBlobberId, args[2].Value)
			}).WithArgs(mockAllocationId).WithReply(
				[]map[string]interface{}{{
					"path":          "",
					"client_id":     p.payerId.ClientID,
					"blobber_id":    mockBlobberId,
					"allocation_id": mockAllocationId,
					"balance":       mockBigBalance,
					"expire_at":     mockLongTimeInFuture,
				}},
			)

			mocket.Catcher.NewMock().OneTime().WithQuery(
				`SELECT * FROM "marketplace_share_info" WHERE`,
			).WithArgs(
				mockClient.ClientID, p.inData.pathHash,
			).WithReply(
				[]map[string]interface{}{{
					"revoked": p.isRevoked,
				}},
			)
		}
	}

	setupOutMock := func(
		t *testing.T,
		p parameters,
		rm marker.ReadMarker,
	) {
		mocket.Catcher.NewMock().WithQuery(
			`DELETE FROM "read_pools" WHERE`,
		).WithCallback(func(par1 string, args []driver.NamedValue) {
			require.EqualValues(t, p.payerId.ClientID, args[0].Value)
			require.EqualValues(t, mockAllocationId, args[1].Value)
			require.EqualValues(t, mockBlobberId, args[2].Value)
		}).WithID(17)

		var funds int64
		if p.isFunded0Chain || p.isFundedBlobber {
			funds = mockBigBalance
		}
		mocket.Catcher.NewMock().WithQuery(
			`INSERT INTO "read_pools"`,
		).WithCallback(func(par1 string, args []driver.NamedValue) {
			require.EqualValues(t, mockPoolId, args[0].Value)
			require.EqualValues(t, p.payerId.ClientID, args[1].Value)
			require.EqualValues(t, mockBlobberId, args[2].Value)
			require.EqualValues(t, mockAllocationId, args[3].Value)
			require.EqualValues(t, funds, args[4].Value)
			require.EqualValues(t, mockLongTimeInFuture, args[5].Value)
		}).WithID(23)

		mocket.Catcher.NewMock().WithCallback(func(par1 string, args []driver.NamedValue) {
			require.EqualValues(t, client.GetClientID(), args[0].Value)
			require.EqualValues(t, client.GetClientPublicKey(), args[1].Value)
			require.EqualValues(t, mockBlobberId, args[2].Value)
			require.EqualValues(t, mockAllocationId, args[3].Value)
			require.EqualValues(t, mockOwner.ClientID, args[4].Value)
			require.EqualValues(t, now, args[5].Value)
			require.EqualValues(t, p.inData.numBlocks, args[6].Value)
		}).WithQuery(`INSERT INTO "read_markers"`).WithID(11)

		mocket.Catcher.NewMock().WithCallback(func(par1 string, args []driver.NamedValue) {
			//require.EqualValues(t, p.payerId.ClientKey, args[0].Value)
			require.EqualValues(t, client.GetClientPublicKey(), args[0].Value)
			require.EqualValues(t, mockBlobberId, args[1].Value)
			require.EqualValues(t, mockAllocationId, args[2].Value)
			require.EqualValues(t, p.payerId.ClientID, args[3].Value)
			require.EqualValues(t, now, args[4].Value)
			require.EqualValues(t, p.inData.numBlocks, args[5].Value)
			require.EqualValues(t, p.payerId.ClientID, args[7].Value)
		}).WithQuery(`UPDATE "read_markers" SET`).WithID(1)

		mocket.Catcher.NewMock().WithCallback(func(par1 string, par2 []driver.NamedValue) {
			fmt.Println("string", par1, "[]NamedValue", par2)
		}).WithQuery(`UPDATE "file_stats" SET`).WithID(1)
	}

	setupCtx := func(p parameters) context.Context {
		ctx := context.TODO()
		ctx = context.WithValue(ctx, constants.CLIENT_CONTEXT_KEY, client.GetClientID())
		ctx = context.WithValue(ctx, constants.ALLOCATION_CONTEXT_KEY, p.inData.allocationTx)
		ctx = context.WithValue(ctx, constants.CLIENT_KEY_CONTEXT_KEY, client.GetClientPublicKey())

		db := datastore.GetStore().GetDB().Begin()
		ctx = context.WithValue(ctx, datastore.CONNECTION_CONTEXT_KEY, db)
		return ctx
	}

	setupRequest := func(p parameters) (*http.Request, *marker.ReadMarker) {
		body := new(bytes.Buffer)
		formWriter := multipart.NewWriter(body)
		rm := addToForm(t, formWriter, p)
		url := fmt.Sprintf("%s%s%s", "my_blobber_url", "/v1/file/download/", p.allocation.Tx)
		url = url
		req := httptest.NewRequest(http.MethodPost, "/v1/file/download/", body)
		require.NoError(t, req.ParseForm())
		req.Header.Add("Content-Type", formWriter.FormDataContentType())
		return req, rm
	}

	setupParams := func(p *parameters) {
		p.inData = blockDownloadRequest{
			allocationID:   mockAllocationId,
			allocationTx:   mockAllocationTx,
			blobber:        &blockchain.StorageNode{ID: mockBlobberId},
			remotefilepath: mockRemoteFilePath,
			blockNum:       mockBlockNumber,
			encryptedKey:   mockEncryptKey,
			contentMode:    "",
			numBlocks:      10240,
			rxPay:          p.rxPay,
		}
		p.allocation = allocation.Allocation{
			ID: mockAllocationId,
			Tx: mockAllocationTx,
		}
		require.True(t, (p.isOwner && !p.isCollaborator && !p.useAuthTicket) || !p.isOwner)
		p.inData.pathHash = fileref.GetReferenceLookup(p.inData.allocationID, p.inData.remotefilepath)
		p.payerId = getPayerId(*p)
	}

	tests := []test{
		{
			name: "ok_owner_funded_blobber",
			parameters: parameters{
				isOwner:         true,
				isCollaborator:  false,
				useAuthTicket:   false,
				attribute:       common.WhoPays3rdParty,
				isRevoked:       false,
				isFundedBlobber: true,
				isFunded0Chain:  false,
				rxPay:           false,
			},
		},
		{
			name: "ok_owner_funded_0chain",
			parameters: parameters{
				isOwner:         true,
				isCollaborator:  false,
				useAuthTicket:   false,
				attribute:       common.WhoPays3rdParty,
				isRevoked:       false,
				isFundedBlobber: false,
				isFunded0Chain:  true,
				rxPay:           false,
			},
		},
		{
			name: "err_owner_not_funded",
			parameters: parameters{
				isOwner:         true,
				isCollaborator:  false,
				useAuthTicket:   false,
				attribute:       common.WhoPays3rdParty,
				isRevoked:       false,
				isFundedBlobber: false,
				isFunded0Chain:  false,
				rxPay:           false,
			},
			want: want{
				err:    true,
				errMsg: "download_file: pre-redeeming read marker: read_pre_redeem: not enough tokens in client's read pools associated with the allocation->blobber",
			},
		},
		{
			name: "ok_collaborator",
			parameters: parameters{
				isOwner:         false,
				isCollaborator:  true,
				useAuthTicket:   false,
				attribute:       common.WhoPays3rdParty,
				isRevoked:       false,
				isFundedBlobber: true,
				isFunded0Chain:  true,
				rxPay:           false,
			},
		}, /*
			{
				name: "ok_authTicket_wp_owner",
				parameters: parameters{
					isOwner:         false,
					isCollaborator:  true,
					useAuthTicket:   false,
					attribute:       common.WhoPaysOwner,
					isRevoked:       false,
					isFundedBlobber: true,
					isFunded0Chain:  true,
					rxPay:           false,
				},
			},*/
	}
	for _, test := range tests {
		t.Run(test.name,
			func(t *testing.T) {
				setupParams(&test.parameters)
				if test.parameters.isOwner {
					require.NoError(t, client.PopulateClient(mockOwnerWallet, "bls0chain"))
				} else {
					require.NoError(t, client.PopulateClient(mockClientWallet, "bls0chain"))
				}
				transaction.MakeSCRestAPICall = makeMockMakeSCRestAPICall(t, test.parameters)
				request, rm := setupRequest(test.parameters)
				datastore.MocketTheStore(t, mocketLogging)
				setupInMock(t, test.parameters, *rm)
				setupOutMock(t, test.parameters, *rm)

				var sh StorageHandler
				resp, err := sh.DownloadFile(setupCtx(test.parameters), request)

				fmt.Println("err", err)
				fmt.Println("resp", resp)
				require.EqualValues(t, test.want.err, err != nil)
				if err != nil {
					require.EqualValues(t, test.want.errMsg, err.Error())
					return
				}

			},
		)
	}
}
