package handler

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"

	"0chain.net/core/encryption"

	"0chain.net/blobbercore/stats"

	"0chain.net/blobbercore/reference"

	"0chain.net/blobbercore/mocks"

	"github.com/stretchr/testify/assert"

	"0chain.net/blobbercore/allocation"

	"github.com/stretchr/testify/mock"

	"0chain.net/blobbercore/blobbergrpc"
)

func TestBlobberGRPCService_GetAllocation_Success(t *testing.T) {
	req := &blobbergrpc.GetAllocationRequest{
		Context: &blobbergrpc.RequestContext{},
		Id:      "something",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Id, false).Return(&allocation.Allocation{
		Tx: req.Id,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	allocation, err := svc.GetAllocation(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, allocation.Allocation.Tx, req.Id)
}

func TestBlobberGRPCService_GetAllocation_invalidAllocation(t *testing.T) {
	req := &blobbergrpc.GetAllocationRequest{
		Context: &blobbergrpc.RequestContext{},
		Id:      "invalid_allocation",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Id, false).Return(nil, errors.New("some error"))

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetAllocation(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	assert.Equal(t, err.Error(), "some error")
}

func TestBlobberGRPCService_GetFileMetaData_Success(t *testing.T) {
	req := &blobbergrpc.GetFileMetaDataRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "client",
			ClientKey:  "",
			Allocation: "something",
		},
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "testval",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, true).Return(&allocation.Allocation{
		ID: "allocationId",
		Tx: req.Allocation,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.FILE,
	}, nil)
	mockReferencePackage.On("GetCommitMetaTxns", mock.Anything, mock.Anything).Return(nil, nil)
	mockReferencePackage.On("GetCollaborators", mock.Anything, mock.Anything).Return([]reference.Collaborator{
		reference.Collaborator{
			RefID:    1,
			ClientID: "test",
		},
	}, nil)
	mockReferencePackage.On("IsACollaborator", mock.Anything, mock.Anything, mock.Anything).Return(true)
	mockStorageHandler.On("verifyAuthTicket", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetFileMetaData(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.MetaData.FileMetaData.Name, "test")
}

func TestBlobberGRPCService_GetFileMetaData_FileNotExist(t *testing.T) {
	req := &blobbergrpc.GetFileMetaDataRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "client",
			ClientKey:  "",
			Allocation: "something",
		},
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "testval",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, true).Return(&allocation.Allocation{
		ID: "allocationId",
		Tx: req.Allocation,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("file doesnt exist"))
	mockReferencePackage.On("GetCommitMetaTxns", mock.Anything, mock.Anything).Return(nil, nil)
	mockReferencePackage.On("GetCollaborators", mock.Anything, mock.Anything).Return([]reference.Collaborator{
		reference.Collaborator{
			RefID:    1,
			ClientID: "test",
		},
	}, nil)
	mockReferencePackage.On("IsACollaborator", mock.Anything, mock.Anything, mock.Anything).Return(true)
	mockStorageHandler.On("verifyAuthTicket", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetFileMetaData(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func randString(n int) string {

	const hexLetters = "abcdef0123456789"

	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(hexLetters[rand.Intn(len(hexLetters))])
	}
	return sb.String()
}

func TestBlobberGRPCService_GetFileStats_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetFileStatsRequest{
		Context: &blobbergrpc.RequestContext{
			Client:          "owner",
			ClientKey:       "",
			Allocation:      allocationTx,
			ClientSignature: clientSignature,
		},
		Path:       "path",
		PathHash:   "path_hash",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, true).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Context.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		ID:   123,
		Name: "test",
		Type: reference.FILE,
	}, nil)
	mockReferencePackage.On("GetFileStats", mock.Anything, int64(123)).Return(&stats.FileStats{
		NumBlockDownloads: 10,
	}, nil)
	mockReferencePackage.On("GetWriteMarkerEntity", mock.Anything, mock.Anything).Return(nil, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetFileStats(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.MetaData.FileMetaData.Name, "test")
	assert.Equal(t, resp.Stats.NumBlockDownloads, int64(10))
}

func TestBlobberGRPCService_GetFileStats_FileNotExist(t *testing.T) {
	req := &blobbergrpc.GetFileStatsRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "owner",
			ClientKey:  "",
			Allocation: "",
		},
		Path:       "path",
		PathHash:   "path_hash",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:      "allocationId",
		Tx:      req.Allocation,
		OwnerID: "owner",
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("file does not exist"))
	mockReferencePackage.On("GetFileStats", mock.Anything, int64(123)).Return(&stats.FileStats{
		NumBlockDownloads: 10,
	}, nil)
	mockReferencePackage.On("GetWriteMarkerEntity", mock.Anything, mock.Anything).Return(nil, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetFileStats(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_ListEntities_Success(t *testing.T) {
	req := &blobbergrpc.ListEntitiesRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "client",
			ClientKey:  "",
			Allocation: "",
		},
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "something",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		AllocationRoot: "/allocationroot",
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.FILE,
	}, nil)
	mockStorageHandler.On("verifyAuthTicket", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	mockReferencePackage.On("GetRefWithChildren", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.DIRECTORY,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.ListEntities(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.AllocationRoot, "/allocationroot")
}

func TestBlobberGRPCService_ListEntities_InvalidAuthTicket(t *testing.T) {
	req := &blobbergrpc.ListEntitiesRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "client",
			ClientKey:  "",
			Allocation: "",
		},
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "something",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:      "allocationId",
		Tx:      req.Allocation,
		OwnerID: "owner",
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.FILE,
	}, nil)
	mockStorageHandler.On("verifyAuthTicket", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	mockReferencePackage.On("GetRefWithChildren", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.DIRECTORY,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.ListEntities(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_GetObjectPath_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetObjectPathRequest{
		Context: &blobbergrpc.RequestContext{
			Client:          "owner",
			ClientKey:       "",
			Allocation:      allocationTx,
			ClientSignature: clientSignature,
		},
		Allocation: "",
		Path:       "path",
		BlockNum:   "120",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Context.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetObjectPath", mock.Anything, mock.Anything, mock.Anything).Return(&reference.ObjectPath{
		RootHash:     "hash",
		FileBlockNum: 120,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetObjectPath(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.ObjectPath.RootHash, "hash")
	assert.Equal(t, resp.ObjectPath.FileBlockNum, int64(120))

}

func TestBlobberGRPCService_GetObjectPath_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.GetObjectPathRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "owner",
			ClientKey:  "",
			Allocation: "",
		},
		Allocation: "",
		Path:       "path",
		BlockNum:   "120",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(nil, errors.New("invalid allocation"))
	mockReferencePackage.On("GetObjectPathGRPC", mock.Anything, mock.Anything, mock.Anything).Return(&blobbergrpc.ObjectPath{
		RootHash:     "hash",
		FileBlockNum: 120,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetObjectPath(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_GetReferencePath_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetReferencePathRequest{
		Context: &blobbergrpc.RequestContext{
			Client:          "client",
			ClientKey:       "",
			Allocation:      allocationTx,
			ClientSignature: clientSignature,
		},
		Paths:      `["something"]`,
		Path:       "",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Context.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferencePathFromPaths", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name:     "test",
		Type:     reference.DIRECTORY,
		Children: []*reference.Ref{{Name: "test1", Type: reference.FILE}},
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetReferencePath(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.ReferencePath.MetaData.DirMetaData.Name, "test")

}

func TestBlobberGRPCService_GetReferencePath_InvalidPaths(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetReferencePathRequest{
		Context: &blobbergrpc.RequestContext{
			Client:          "client",
			ClientKey:       "",
			Allocation:      allocationTx,
			ClientSignature: clientSignature,
		},
		Paths:      `["something"]`,
		Path:       "",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Context.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferencePathFromPaths", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("invalid paths"))

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetReferencePath(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	assert.Equal(t, err.Error(), "invalid paths")

}

func TestBlobberGRPCService_GetObjectTree_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetObjectTreeRequest{
		Context: &blobbergrpc.RequestContext{
			Client:          "owner",
			ClientKey:       "",
			Allocation:      allocationTx,
			ClientSignature: clientSignature,
		},
		Path:       "something",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Context.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetObjectTree", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name:     "test",
		Type:     reference.DIRECTORY,
		Children: []*reference.Ref{{Name: "test1", Type: reference.FILE}},
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetObjectTree(context.Background(), req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, resp.ReferencePath.MetaData.DirMetaData.Name, "test")

}

func TestBlobberGRPCService_GetObjectTree_NotOwner(t *testing.T) {
	req := &blobbergrpc.GetObjectTreeRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     "hacker",
			ClientKey:  "",
			Allocation: "",
		},
		Path:       "something",
		Allocation: "",
	}

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:      "allocationId",
		Tx:      req.Allocation,
		OwnerID: "owner",
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetObjectTree(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

}
