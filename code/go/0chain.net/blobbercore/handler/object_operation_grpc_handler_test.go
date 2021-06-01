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
	"path/filepath"
	"testing"
)

func TestBlobberGRPCService_CopyObject_Success(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CopyObjectRequest{
		Allocation:   allocationTx,
		Path:         "path",
		ConnectionId: "connection_id",
		Dest:         "dest",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "client_key",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	alloc := &allocation.Allocation{
		Tx:             req.Allocation,
		ID:             req.Allocation,
		OwnerID:        `client`,
		OwnerPublicKey: pubKey,
	}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(alloc, nil)

	mockReferencePackage := &mocks.PackageHandler{}
	allocChange := &allocation.AllocationChangeCollector{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, alloc.OwnerID).Return(allocChange, nil)

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
	newPath := filepath.Join(req.Dest, objectRef.Name)
	mockReferencePackage.On(`GetReference`, mock.Anything, alloc.ID, newPath).
		Return(nil, nil)
	mockReferencePackage.On(`GetReference`, mock.Anything, alloc.ID, req.Dest).
		Return(&reference.Ref{Type: `d`}, nil)
	mockReferencePackage.On(`SaveAllocationChanges`, mock.Anything, allocChange).
		Return(nil)

	resOk := &blobbergrpc.CopyObjectResponse{
		Filename:     objectRef.Name,
		Size:         objectRef.Size,
		ContentHash:  objectRef.Hash,
		MerkleRoot:   objectRef.MerkleRoot,
		UploadLength: 0,
		UploadOffset: 0,
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.CopyObject(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)
}

func TestBlobberGRPCService_CopyObject_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.CopyObjectRequest{
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

	_, err := svc.CopyObject(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "invalid_parameters: Invalid allocation id passed.some error" {
		t.Fatal(`unexpected error - `, err)
	}
}
