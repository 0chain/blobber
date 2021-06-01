package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	"testing"
)

func TestBlobberGRPCService_DownloadFile_Success(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.DownloadFileRequest{
		Allocation: allocationTx,
		Path:       `path`,
		RxPay:      "false",
		NumBlocks:  "5",
		BlockNum:   "5",
		ReadMarker: `{}`,
		AuthToken:  "",
		Content:    "",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "client_key",
		common.ClientSignatureHeader: clientSignature,
	}))
	ctx = setupGRPCHandlerContext(ctx, GetGRPCMetaDataFromCtx(ctx), req.Allocation)

	alloc := &allocation.Allocation{
		Tx:             req.Allocation,
		ID:             `allocation_id`,
		OwnerID:        `client`,
		OwnerPublicKey: pubKey,
	}

	rm := &readmarker.ReadMarker{
		AllocationID:    alloc.ID,
		ClientPublicKey: `client_key`,
		ClientID:        `client`,
		Signature:       clientSignature,
		OwnerID:         alloc.OwnerID,
	}
	rmStr, _ := json.Marshal(rm)
	req.ReadMarker = string(rmStr)

	//var pentBlocksNum = int64(10)
	var latestRm = &readmarker.ReadMarker{}

	mockStorageHandler := &storageHandlerI{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(alloc, nil)
	mockStorageHandler.On(`readPreRedeem`, mock.Anything, alloc, int64(5), int64(0), alloc.OwnerID).Return(
		nil)

	mockFileStore := &mocks.FileStore{}
	mockFileStore.On(`GetFileBlock`, alloc.ID, mock.Anything, int64(5), int64(5)).Return(
		[]byte{}, nil)
	mockReferencePackage := &mocks.PackageHandler{}
	pathHash := req.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, alloc.ID, req.Path).
		Return(pathHash)
	mockReferencePackage.On(`VerifyReadMarker`, mock.Anything, mock.Anything, alloc).
		Return(nil)

	objectRef := &reference.Ref{
		Name:        "test",
		ID:          123,
		ContentHash: `hash`,
		MerkleRoot:  `root`,
		Size:        1,
		Type:        reference.FILE,
	}

	rme := &readmarker.ReadMarkerEntity{
		RedeemRequired: false,
		LatestRM:       latestRm,
	}

	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, alloc.ID, pathHash).
		Return(objectRef, nil)
	mockReferencePackage.On(`IsACollaborator`, mock.Anything, objectRef.ID, alloc.OwnerID).
		Return(true)
	mockReferencePackage.On(`GetLatestReadMarkerEntity`, mock.Anything, alloc.OwnerID).
		Return(rme, nil)
	mockReferencePackage.On(`GetFileStore`).
		Return(mockFileStore)
	mockReferencePackage.On(`SaveLatestReadMarker`, mock.Anything, mock.Anything, false).
		Return(nil)
	mockReferencePackage.On(`FileBlockDownloaded`, mock.Anything, objectRef.ID).
		Return()
	mockReferencePackage.On(`GetNewReadMaker`, mock.Anything).Return(rme)

	resOk := &blobbergrpc.DownloadFileResponse{
		Success:  false,
		LatestRm: convert.ReadMarkerToReadMarkerGRPC(latestRm),
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.DownloadFile(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)
}

func TestBlobberGRPCService_DownloadFile_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.DownloadFileRequest{
		Allocation: `invalid_allocation`,
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "client_key",
		common.ClientSignatureHeader: "clientSignature",
	}))

	mockStorageHandler := &storageHandlerI{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(nil, errors.New("some error"))

	mockReferencePackage := &mocks.PackageHandler{}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.DownloadFile(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid_parameters: Invalid allocation id passed.some error" {
		t.Fatal(`unexpected error - `, err)
	}
}
