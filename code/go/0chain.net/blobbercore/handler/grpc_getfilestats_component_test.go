package handler

// func TestBlobberGRPCService_GetFilestats(t *testing.T) {
// 	bClient, tdController := setupGrpcTests(t)
// 	allocationTx := randString(32)

// 	pubKey, _, signScheme := GeneratePubPrivateKey(t)
// 	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

// 	err := tdController.AddGetFileStatsTestData(allocationTx, pubKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	testCases := []struct {
// 		name             string
// 		context          metadata.MD
// 		input            *blobbergrpc.GetFileStatsRequest
// 		expectedFileName string
// 		expectingError   bool
// 	}{
// 		{
// 			name: "Success",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetFileStatsRequest{
// 				Path:       "examplePath",
// 				PathHash:   "exampleId:examplePath",
// 				Allocation: allocationTx,
// 			},
// 			expectedFileName: "filename",
// 			expectingError:   false,
// 		},
// 		{
// 			name: "Unknown Path",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetFileStatsRequest{
// 				Path:       "examplePath",
// 				PathHash:   "exampleId:examplePath123",
// 				Allocation: allocationTx,
// 			},
// 			expectedFileName: "",
// 			expectingError:   true,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		ctx := context.Background()
// 		ctx = metadata.NewOutgoingContext(ctx, tc.context)
// 		getFileStatsResp, err := bClient.GetFileStats(ctx, tc.input)
// 		if err != nil {
// 			if !tc.expectingError {
// 				t.Fatal(err)
// 			}
// 			continue
// 		}

// 		if tc.expectingError {
// 			t.Fatal("expected error")
// 		}

// 		if getFileStatsResp.MetaData.FileMetaData.Name != tc.expectedFileName {
// 			t.Fatal("unexpected file name from GetFileStats rpc")
// 		}
// 	}
// }
