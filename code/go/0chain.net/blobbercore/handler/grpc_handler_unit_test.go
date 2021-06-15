package handler

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

func randString(n int) string {

	const hexLetters = "abcdef0123456789"

	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteByte(hexLetters[rand.Intn(len(hexLetters))])
	}
	return sb.String()
}

func TestBlobberGRPCService_ListEntities_Success(t *testing.T) {
	req := &blobbergrpc.ListEntitiesRequest{
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "something",
		Allocation: "",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: "",
	}))

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
	resp, err := svc.ListEntities(ctx, req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.AllocationRoot, "/allocationroot")
}

func TestBlobberGRPCService_ListEntities_InvalidAuthTicket(t *testing.T) {
	req := &blobbergrpc.ListEntitiesRequest{
		Path:       "path",
		PathHash:   "path_hash",
		AuthToken:  "something",
		Allocation: "",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: "",
	}))

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
	_, err := svc.ListEntities(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_GetObjectPath_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetObjectPathRequest{
		Allocation: allocationTx,
		Path:       "path",
		BlockNum:   "120",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetObjectPath", mock.Anything, mock.Anything, mock.Anything).Return(&reference.ObjectPath{
		RootHash:     "hash",
		FileBlockNum: 120,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetObjectPath(ctx, req)
	if err != nil {
		t.Fatal("unexpected error")
	}

	assert.Equal(t, resp.ObjectPath.RootHash, "hash")
	assert.Equal(t, resp.ObjectPath.FileBlockNum, int64(120))

}

func TestBlobberGRPCService_GetObjectPath_InvalidAllocation(t *testing.T) {
	req := &blobbergrpc.GetObjectPathRequest{
		Allocation: "",
		Path:       "path",
		BlockNum:   "120",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: "",
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(nil, errors.New("invalid allocation"))
	mockReferencePackage.On("GetObjectPathGRPC", mock.Anything, mock.Anything, mock.Anything).Return(&blobbergrpc.ObjectPath{
		RootHash:     "hash",
		FileBlockNum: 120,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetObjectPath(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_GetReferencePath_Success(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.GetReferencePathRequest{
		Paths:      `["something"]`,
		Path:       "",
		Allocation: allocationTx,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferencePathFromPaths", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name:     "test",
		Type:     reference.DIRECTORY,
		Children: []*reference.Ref{{Name: "test1", Type: reference.FILE}},
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetReferencePath(ctx, req)
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
		Paths:      `["something"]`,
		Path:       "",
		Allocation: allocationTx,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "client",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferencePathFromPaths", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("invalid paths"))

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetReferencePath(ctx, req)
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
		Path:       "something",
		Allocation: allocationTx,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetObjectTree", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name:     "test",
		Type:     reference.DIRECTORY,
		Children: []*reference.Ref{{Name: "test1", Type: reference.FILE}},
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.GetObjectTree(ctx, req)
	if err != nil {
		t.Fatal("unexpected error - " + err.Error())
	}

	assert.Equal(t, resp.ReferencePath.MetaData.DirMetaData.Name, "test")

}

func TestBlobberGRPCService_GetObjectTree_NotOwner(t *testing.T) {
	req := &blobbergrpc.GetObjectTreeRequest{
		Path:       "something",
		Allocation: "",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "hacker",
		common.ClientKeyHeader:       "",
		common.ClientSignatureHeader: "",
	}))

	mockStorageHandler := &storageHandlerI{}
	mockReferencePackage := &mocks.PackageHandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:      "allocationId",
		Tx:      req.Allocation,
		OwnerID: "owner",
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.GetObjectTree(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_CalculateHashSuccess(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CalculateHashRequest{
		Allocation: allocationTx,
		Path:       "some-path",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferencePathFromPaths", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.DIRECTORY,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.CalculateHash(ctx, req)
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}

	assert.Equal(t, resp.GetMessage(), "Hash recalculated for the given paths")
}

func TestBlobberGRPCService_CalculateHashNotOwner(t *testing.T) {
	req := &blobbergrpc.CalculateHashRequest{
		Allocation: "",
		Path:       "some-path",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "hacker",
		common.ClientSignatureHeader: "",
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(&allocation.Allocation{
		ID:      "allocationId",
		Tx:      req.Allocation,
		OwnerID: "owner",
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.CalculateHash(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_CommitMetaTxnSuccess(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CommitMetaTxnRequest{
		Path:       "/some_file",
		PathHash:   "exampleId:examplePath",
		AuthToken:  "",
		Allocation: allocationTx,
		TxnId:      "8",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:             "8",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		ID:   8,
		Type: reference.FILE,
	}, nil)
	mockReferencePackage.On("AddCommitMetaTxn", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.CommitMetaTxn(ctx, req)
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}

	assert.Equal(t, resp.GetMessage(), "Added commitMetaTxn successfully")
}

func TestBlobberGRPCService_CommitMetaTxnError(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CommitMetaTxnRequest{
		Path:       "/some_file",
		PathHash:   "exampleId:examplePath",
		AuthToken:  "",
		Allocation: allocationTx,
		TxnId:      "", // TxnId not passed, expecting error
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "owner",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:             "8",
		Tx:             req.Allocation,
		OwnerID:        "owner",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		ID:   8,
		Type: reference.FILE,
	}, nil)
	mockReferencePackage.On("AddCommitMetaTxn", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.CommitMetaTxn(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlobberGRPCService_AddCollaboratorSuccess(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CollaboratorRequest{
		Allocation: allocationTx,
		Path:       "some-path",
		CollabId:   "12",
		Method:     http.MethodPost,
		PathHash:   "exampleId:examplePath",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "12",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "12",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.FILE,
	}, nil)
	mockReferencePackage.On("IsACollaborator", mock.Anything, mock.Anything, mock.Anything).
		Return(false)
	mockReferencePackage.On("AddCollaborator", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	resp, err := svc.Collaborator(ctx, req)
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}

	assert.Equal(t, resp.GetMessage(), "Added collaborator successfully")
}

func TestBlobberGRPCService_AddCollaboratorError(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	req := &blobbergrpc.CollaboratorRequest{
		Allocation: allocationTx,
		Path:       "some-path",
		CollabId:   "12",
		Method:     http.MethodPost,
		PathHash:   "exampleId:examplePath",
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          "1",
		common.ClientSignatureHeader: clientSignature,
	}))

	mockStorageHandler := new(storageHandlerI)
	mockReferencePackage := new(mocks.PackageHandler)
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, true).Return(&allocation.Allocation{
		ID:             "allocationId",
		Tx:             req.Allocation,
		OwnerID:        "12",
		OwnerPublicKey: pubKey,
	}, nil)
	mockReferencePackage.On("GetReferenceFromLookupHash", mock.Anything, mock.Anything, mock.Anything).Return(&reference.Ref{
		Name: "test",
		Type: reference.FILE,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler, mockReferencePackage)
	_, err := svc.Collaborator(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}
