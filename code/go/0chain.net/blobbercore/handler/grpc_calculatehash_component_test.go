package handler

// func TestBlobberGRPCService_CalculateHash(t *testing.T) {
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
// 		input          *blobbergrpc.CalculateHashRequest
// 		expectingError bool
// 	}{
// 		{
// 			name: "Success",
// 			context: metadata.New(map[string]string{
// 				common.ClientHeader:          "exampleOwnerId",
// 				common.ClientSignatureHeader: clientSignature,
// 				common.ClientKeyHeader:       pubKey,
// 			}),
// 			input: &blobbergrpc.CalculateHashRequest{
// 				Paths:      "",
// 				Path:       "/",
// 				Allocation: allocationTx,
// 			},
// 			expectingError: false,
// 		},
// 	}

// 	for _, tc := range testCases {
// 		ctx := context.Background()
// 		ctx = metadata.NewOutgoingContext(ctx, tc.context)
// 		_, err := bClient.CalculateHash(ctx, tc.input)
// 		if err != nil {
// 			if !tc.expectingError {
// 				t.Fatal(err)
// 			}

// 			continue
// 		}

// 		if tc.expectingError {
// 			t.Fatal("expected error")
// 		}
// 	}
// }
