package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/reference"

	"github.com/stretchr/testify/mock"

	"0chain.net/blobbercore/blobbergrpc"
)

type mockStoragehandler struct {
	mock.Mock
}

func (m *mockStoragehandler) verifyAllocation(ctx context.Context, tx string, readonly bool) (alloc *allocation.Allocation, err error) {
	args := m.Called(ctx, tx, readonly)
	arg1, _ := args.Get(0).(*allocation.Allocation)
	return arg1, args.Error(1)
}

func (m *mockStoragehandler) verifyAuthTicket(ctx context.Context, authTokenString string, allocationObj *allocation.Allocation, refRequested *reference.Ref, clientID string) (bool, error) {
	args := m.Called(ctx, authTokenString, allocationObj, refRequested, clientID)
	return args.Bool(0), args.Error(1)
}

func TestBlobberGRPCService_GetAllocation_Success(t *testing.T) {
	req := &blobbergrpc.GetAllocationRequest{
		Context: &blobbergrpc.RequestContext{},
		Id:      "something",
	}

	mockStorageHandler := &mockStoragehandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Id, false).Return(&allocation.Allocation{
		Tx: req.Id,
	}, nil)

	svc := newGRPCBlobberService(mockStorageHandler)
	allocation, err := svc.GetAllocation(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, allocation.Allocation.Tx, req.Id)
}

func TestBlobberGRPCService_GetAllocation_verifyAllocation_Error(t *testing.T) {
	req := &blobbergrpc.GetAllocationRequest{
		Context: &blobbergrpc.RequestContext{},
		Id:      "something",
	}

	mockStorageHandler := &mockStoragehandler{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Id, false).Return(nil, errors.New("some error"))

	svc := newGRPCBlobberService(mockStorageHandler)
	_, err := svc.GetAllocation(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}

	assert.Equal(t, err.Error(), "some error")
}
