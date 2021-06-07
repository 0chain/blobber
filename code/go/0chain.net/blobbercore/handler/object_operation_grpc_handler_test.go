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

func TestBlobberGRPCService_UpdateObjectAttributes_Success(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	req := &blobbergrpc.UpdateObjectAttributesRequest{
		Allocation:   allocationTx,
		Attributes:   `{"who_pays_for_reads" : 1}`,
		Path:         `path`,
		ConnectionId: `connection_id`,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "client_key",
		common.ClientSignatureHeader: clientSignature,
	}))

	resOk := &blobbergrpc.UpdateObjectAttributesResponse{WhoPaysForReads: int64(1)}

	mockStorageHandler := &storageHandlerI{}
	alloc := &allocation.Allocation{
		Tx:             req.Allocation,
		ID:             `allocation_id`,
		OwnerID:        `client`,
		OwnerPublicKey: pubKey,
	}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(alloc, nil)

	mockReferencePackage := &mocks.PackageHandler{}
	allocChange := &allocation.AllocationChangeCollector{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, `client`).Return(allocChange, nil)
	mockReferencePackage.On(`SaveAllocationChanges`, mock.Anything, allocChange).
		Return(nil)

	pathHash := req.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, alloc.ID, req.Path).
		Return(pathHash)

	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, alloc.ID, pathHash).Return(
		&reference.Ref{
			Name: "test",
			Type: reference.FILE,
		}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.UpdateObjectAttributes(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)
}

func TestBlobberGRPCService_UpdateObjectAttributes_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.UpdateObjectAttributesRequest{
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
	_, err := svc.UpdateObjectAttributes(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid_parameters: Invalid allocation id passed.some error" {
		t.Fatal(`unexpected error - `, err)
	}
}
