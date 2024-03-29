package handler

// func TestBlobberGRPCService_GetReferencePath(t *testing.T) {
// 	bClient, tdController := setupGrpcTests(t)
// 	allocationTx := randString(32)

// 	pubKey, _, signScheme := GeneratePubPrivateKey(t)
// 	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

// 	err := tdController.AddGetReferencePathTestData(allocationTx, pubKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	testCases := []struct {
// 		name           string
// 		context        metadata.MD
// 		input          *blobbergrpc.GetReferencePathRequest
// 		expectedPath   string
// 		expectingError bool
// 	}{
// 		{
// 			name: "Success",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 			}),
// 			input: &blobbergrpc.GetReferencePathRequest{
// 				Paths:      "",
// 				Path:       "/",
// 				Allocation: allocationTx,
// 			},
// 			expectedPath:   "/",
// 			expectingError: false,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		ctx := context.Background()
// 		ctx = metadata.NewOutgoingContext(ctx, tc.context)
// 		getReferencePathResp, err := bClient.GetReferencePath(ctx, tc.input)
// 		if err != nil {
// 			if !tc.expectingError {
// 				t.Fatal(err)
// 			}
// 			continue
// 		}

// 		if tc.expectingError {
// 			t.Fatal("expected error")
// 		}

// 		if getReferencePathResp.ReferencePath.MetaData.DirMetaData.Path != tc.expectedPath {
// 			t.Fatal("unexpected path from GetReferencePath rpc")
// 		}
// 	}
// }
