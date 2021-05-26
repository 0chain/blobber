package handler

import (
	"context"
	"errors"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	"testing"
)

func TestBlobberGRPCService_RenameObject_Success(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	req := &blobbergrpc.RenameObjectRequest{
		Allocation:   allocationTx,
		Path:         `path`,
		ConnectionId: `connection_id`,
		NewName:      `new_name`,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "client_key",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	alloc := &allocation.Allocation{
		Tx:             req.Allocation,
		ID:             `allocation_id`,
		OwnerID:        `client`,
		OwnerPublicKey: pubKey,
	}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(alloc, nil)

	mockAllocCollector := &mocks.IAllocationChangeCollector{}
	mockAllocCollector.On(`GetConnectionID`).Return(req.ConnectionId)
	mockAllocCollector.On(`GetAllocationID`).Return(req.Allocation)
	mockAllocCollector.On(`SetSize`, mock.Anything).Return()
	mockAllocCollector.On(`GetSize`).Return(int64(1))
	mockAllocCollector.On(`AddChange`, mock.Anything, mock.Anything).Return()
	mockAllocCollector.On(`Save`, mock.Anything).Return(nil)
	mockAllocCollector.On(`TableName`).Return(`allocation_connections`)

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, `client`).Return(mockAllocCollector, nil)

	pathHash := req.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, alloc.ID, req.Path).
		Return(pathHash)
	objectRef := &reference.Ref{
		Name:        "test",
		ContentHash: `hash`,
		MerkleRoot:  `root`,
		Size:        1,
	}
	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, alloc.ID, pathHash).
		Return(objectRef, nil)

	resOk := &blobbergrpc.RenameObjectResponse{
		Filename:     req.NewName,
		Size:         objectRef.Size,
		ContentHash:  objectRef.Hash,
		MerkleRoot:   objectRef.MerkleRoot,
		UploadLength: 0,
		UploadOffset: 0,
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.RenameObject(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)
}

func TestBlobberGRPCService_RenameObject_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.RenameObjectRequest{
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
	_, err := svc.RenameObject(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid_parameters: Invalid allocation id passed.some error" {
		t.Fatal(`unexpected error - `, err)
	}
}
