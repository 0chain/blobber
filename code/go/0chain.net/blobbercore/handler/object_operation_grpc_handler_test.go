package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	"testing"
)

func TestBlobberGRPCService_WriteFile_Success_POST(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.UploadFileRequest{
		Allocation:   allocationTx,
		Path:         `path`,
		ConnectionId: `connection_id`,
		Method:       `POST`,
		UploadMeta:   `{"filename": "test_file","filepath":"path"}`,
		UploadFile:   []byte(`this is a upload file content`),
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

	mockFileStore := &mocks.FileStore{}
	fileOutput := &filestore.FileOutputData{
		Name:        "test_file",
		Path:        "path",
		MerkleRoot:  "root",
		ContentHash: "hash",
	}
	mockFileStore.On(`WriteFileGRPC`, alloc.ID, mock.Anything, mock.Anything, req.ConnectionId).Return(
		fileOutput, nil)

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, alloc.OwnerID).Return(mockAllocCollector, nil)
	mockReferencePackage.On(`GetReference`, mock.Anything, alloc.ID, `path`).
		Return(nil, nil)
	mockReferencePackage.On(`GetFileStore`).
		Return(mockFileStore)

	resOk := &blobbergrpc.UploadFileResponse{
		Filename:    fileOutput.Name,
		Size:        0,
		ContentHash: fileOutput.ContentHash,
		MerkleRoot:  fileOutput.MerkleRoot,
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.WriteFile(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)

}

func TestBlobberGRPCService_WriteFile_Success_PUT(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.UploadFileRequest{
		Allocation:   allocationTx,
		Path:         `path`,
		ConnectionId: `connection_id`,
		Method:       `PUT`,
		UpdateMeta:   `{"filename": "test_file","filepath":"path"}`,
		UploadFile:   []byte(`this is a upload file content`),
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

	mockFileStore := &mocks.FileStore{}
	fileOutput := &filestore.FileOutputData{
		Name:        "test_file",
		Path:        "path",
		MerkleRoot:  "root",
		ContentHash: "hash",
	}
	mockFileStore.On(`WriteFileGRPC`, alloc.ID, mock.Anything, mock.Anything, req.ConnectionId).Return(
		fileOutput, nil)

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, alloc.OwnerID).Return(mockAllocCollector, nil)
	mockReferencePackage.On(`GetReference`, mock.Anything, alloc.ID, `path`).
		Return(&reference.Ref{}, nil)
	mockReferencePackage.On(`GetFileStore`).
		Return(mockFileStore)

	resOk := &blobbergrpc.UploadFileResponse{
		Filename:    fileOutput.Name,
		Size:        0,
		ContentHash: fileOutput.ContentHash,
		MerkleRoot:  fileOutput.MerkleRoot,
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.WriteFile(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)

}

func TestBlobberGRPCService_WriteFile_Success_DELETE(t *testing.T) {
	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.UploadFileRequest{
		Allocation:   allocationTx,
		Path:         `path`,
		ConnectionId: `connection_id`,
		Method:       `DELETE`,
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
		PayerID:        `client`,
	}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).
		Return(alloc, nil)

	mockAllocCollector := &mocks.IAllocationChangeCollector{}
	mockAllocCollector.On(`GetConnectionID`).Return(req.ConnectionId)
	mockAllocCollector.On(`GetAllocationID`).Return(alloc.ID)
	mockAllocCollector.On(`SetSize`, mock.Anything).Return()
	mockAllocCollector.On(`GetSize`).Return(int64(1))
	mockAllocCollector.On(`AddChange`, mock.Anything, mock.Anything).Return()
	mockAllocCollector.On(`Save`, mock.Anything).Return(nil)
	mockAllocCollector.On(`TableName`).Return(`allocation_connections`)

	mockFileStore := &mocks.FileStore{}
	fileRef := &reference.Ref{
		Name:       "test_file",
		Path:       "path",
		MerkleRoot: "root",
		Hash:       "hash",
	}

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, alloc.OwnerID).Return(mockAllocCollector, nil)
	mockReferencePackage.On(`GetReference`, mock.Anything, alloc.ID, `path`).
		Return(fileRef, nil)
	mockReferencePackage.On(`GetFileStore`).
		Return(mockFileStore)

	resOk := &blobbergrpc.UploadFileResponse{
		Filename:    fileRef.Name,
		Size:        fileRef.Size,
		ContentHash: fileRef.Hash,
		MerkleRoot:  fileRef.MerkleRoot,
	}

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	gotResponse, err := svc.WriteFile(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, gotResponse, resOk)

}
