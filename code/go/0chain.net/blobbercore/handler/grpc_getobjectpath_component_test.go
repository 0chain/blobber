package handler

// func TestBlobberGRPCService_GetObjectPath(t *testing.T) {
// 	bClient, tdController := setupGrpcTests(t)
// 	allocationTx := randString(32)

// 	pubKey, _, signScheme := GeneratePubPrivateKey(t)
// 	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

// 	err := tdController.AddGetObjectPathTestData(allocationTx, pubKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	testCases := []struct {
// 		name           string
// 		context        metadata.MD
// 		input          *blobbergrpc.GetObjectPathRequest
// 		expectedPath   string
// 		expectingError bool
// 	}{
// 		{
// 			name: "Success",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetObjectPathRequest{
// 				Allocation: allocationTx,
// 				Path:       "examplePath",
// 				BlockNum:   "0",
// 			},
// 			expectedPath:   "/",
// 			expectingError: false,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		ctx := context.Background()
// 		ctx = metadata.NewOutgoingContext(ctx, tc.context)
// 		getObjectPathResp, err := bClient.GetObjectPath(ctx, tc.input)
// 		if err != nil {
// 			if !tc.expectingError {
// 				t.Fatal(err)
// 			}
// 			continue
// 		}

// 		if tc.expectingError {
// 			t.Fatal("expected error")
// 		}

// 		if getObjectPathResp.ObjectPath.Path.DirMetaData.Path != tc.expectedPath {
// 			t.Fatal("unexpected root hash from GetObjectPath rpc")
// 		}
// 	}
// }
