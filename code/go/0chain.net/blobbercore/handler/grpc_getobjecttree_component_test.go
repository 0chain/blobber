package handler

// func TestBlobberGRPCService_GetObjectTree(t *testing.T) {
// 	bClient, tdController := setupGrpcTests(t)
// 	allocationTx := randString(32)

// 	pubKey, _, signScheme := GeneratePubPrivateKey(t)
// 	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

// 	err := tdController.AddGetObjectTreeTestData(allocationTx, pubKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	testCases := []struct {
// 		name             string
// 		context          metadata.MD
// 		input            *blobbergrpc.GetObjectTreeRequest
// 		expectedFileName string
// 		expectingError   bool
// 	}{
// 		{
// 			name: "Success",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetObjectTreeRequest{
// 				Path:       "/",
// 				Allocation: allocationTx,
// 			},
// 			expectedFileName: "root",
// 			expectingError:   false,
// 		},
// 		{
// 			name: "bad path",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetObjectTreeRequest{
// 				Path:       "/2",
// 				Allocation: "",
// 			},
// 			expectedFileName: "root",
// 			expectingError:   true,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		ctx := context.Background()
// 		ctx = metadata.NewOutgoingContext(ctx, tc.context)
// 		getObjectTreeResp, err := bClient.GetObjectTree(ctx, tc.input)
// 		if err != nil {
// 			if !tc.expectingError {
// 				t.Fatal(err)
// 			}
// 			continue
// 		}

// 		if tc.expectingError {
// 			t.Fatal("expected error")
// 		}

// 		if getObjectTreeResp.ReferencePath.MetaData.DirMetaData.Name != tc.expectedFileName {
// 			t.Fatal("unexpected root name from GetObject")
// 		}
// 	}
// }
