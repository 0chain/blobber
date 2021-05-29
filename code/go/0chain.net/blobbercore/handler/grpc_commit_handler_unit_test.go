package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

func TestBlobberGRPCService_Commit(t *testing.T) {
	allocationTx := randString(32)

	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))

	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().UnixNano()
	rootRefHash := "someHash"
	clientId := encryption.Hash(pubKeyBytes)
	connectionId := "connection_id"
	allocationId := "allocationId"
	req := &blobbergrpc.CommitRequest{
		Allocation:   allocationTx,
		ConnectionId: connectionId,
		WriteMarker:  `{"allocation_id":"` + allocationId + `","timestamp":` + fmt.Sprint(timestamp) + `,"allocation_root":"` + encryption.Hash(rootRefHash+":"+strconv.FormatInt(int64(timestamp), 10)) + `"}`,
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		common.ClientHeader:          clientId,
		common.ClientKeyHeader:       pubKey,
		common.ClientSignatureHeader: clientSignature,
	}))

	testcases := []struct {
		description                            string
		getAllocationChangesReturn             func() (*allocation.AllocationChangeCollector, error)
		verifyMarkerReturn                     func() error
		applyChangesReturn                     func() error
		getReferenceReturn                     func() (*reference.Ref, error)
		verifyAllocationReturn                 func() (*allocation.Allocation, error)
		getWriteMarkerEntityReturn             func() (*writemarker.WriteMarkerEntity, error)
		updateAllocationAndCommitChangesReturn func() error
		expectedError                          bool
	}{
		{
			description:   "success",
			expectedError: false,
			getAllocationChangesReturn: func() (*allocation.AllocationChangeCollector, error) {
				return &allocation.AllocationChangeCollector{
					ConnectionID: connectionId,
					AllocationID: allocationId,
					ClientID:     "",
					Size:         0,
					Changes: []*allocation.AllocationChange{&allocation.AllocationChange{
						ChangeID:     1,
						Size:         0,
						Operation:    "insert",
						ConnectionID: connectionId,
						Input:        "",
						ModelWithTS:  datastore.ModelWithTS{},
					}},
					AllocationChanges: nil,
					Status:            0,
					ModelWithTS:       datastore.ModelWithTS{},
				}, nil
			},
			verifyMarkerReturn: func() error {
				return nil
			},
			applyChangesReturn: func() error {
				return nil
			},
			getReferenceReturn: func() (*reference.Ref, error) {
				return &reference.Ref{
					Hash: rootRefHash,
				}, nil
			},
			verifyAllocationReturn: func() (*allocation.Allocation, error) {
				return &allocation.Allocation{
					ID:             allocationId,
					Tx:             req.Allocation,
					OwnerID:        clientId,
					OwnerPublicKey: pubKey,
				}, nil
			},
			getWriteMarkerEntityReturn: func() (*writemarker.WriteMarkerEntity, error) {
				return nil, nil
			},
			updateAllocationAndCommitChangesReturn: func() error {
				return nil
			},
		},
		{
			description:   "could not commit",
			expectedError: true,
			getAllocationChangesReturn: func() (*allocation.AllocationChangeCollector, error) {
				return &allocation.AllocationChangeCollector{
					ConnectionID: connectionId,
					AllocationID: allocationId,
					ClientID:     "",
					Size:         0,
					Changes: []*allocation.AllocationChange{&allocation.AllocationChange{
						ChangeID:     1,
						Size:         0,
						Operation:    "insert",
						ConnectionID: connectionId,
						Input:        "",
						ModelWithTS:  datastore.ModelWithTS{},
					}},
					AllocationChanges: nil,
					Status:            0,
					ModelWithTS:       datastore.ModelWithTS{},
				}, nil
			},
			verifyMarkerReturn: func() error {
				return nil
			},
			applyChangesReturn: func() error {
				return nil
			},
			getReferenceReturn: func() (*reference.Ref, error) {
				return &reference.Ref{
					Hash: rootRefHash,
				}, nil
			},
			verifyAllocationReturn: func() (*allocation.Allocation, error) {
				return &allocation.Allocation{
					ID:             allocationId,
					Tx:             req.Allocation,
					OwnerID:        clientId,
					OwnerPublicKey: pubKey,
				}, nil
			},
			getWriteMarkerEntityReturn: func() (*writemarker.WriteMarkerEntity, error) {
				return nil, nil
			},
			updateAllocationAndCommitChangesReturn: func() error {
				return fmt.Errorf("some error")
			},
		},
		{
			description:   "invalid marker",
			expectedError: true,
			getAllocationChangesReturn: func() (*allocation.AllocationChangeCollector, error) {
				return &allocation.AllocationChangeCollector{
					ConnectionID: connectionId,
					AllocationID: allocationId,
					ClientID:     "",
					Size:         0,
					Changes: []*allocation.AllocationChange{&allocation.AllocationChange{
						ChangeID:     1,
						Size:         0,
						Operation:    "insert",
						ConnectionID: connectionId,
						Input:        "",
						ModelWithTS:  datastore.ModelWithTS{},
					}},
					AllocationChanges: nil,
					Status:            0,
					ModelWithTS:       datastore.ModelWithTS{},
				}, nil
			},
			verifyMarkerReturn: func() error {
				return fmt.Errorf("invalid marker")
			},
			applyChangesReturn: func() error {
				return nil
			},
			getReferenceReturn: func() (*reference.Ref, error) {
				return &reference.Ref{
					Hash: rootRefHash,
				}, nil
			},
			verifyAllocationReturn: func() (*allocation.Allocation, error) {
				return &allocation.Allocation{
					ID:             allocationId,
					Tx:             req.Allocation,
					OwnerID:        clientId,
					OwnerPublicKey: pubKey,
				}, nil
			},
			getWriteMarkerEntityReturn: func() (*writemarker.WriteMarkerEntity, error) {
				return nil, nil
			},
			updateAllocationAndCommitChangesReturn: func() error {
				return nil
			},
		},
	}

	for _, tc := range testcases {

		mockStorageHandler := &storageHandlerI{}
		mockPackageHandler := &mocks.PackageHandler{}

		mockPackageHandler.On("GetAllocationChanges", mock.Anything, connectionId, allocationId, mock.Anything).Return(tc.getAllocationChangesReturn())

		mockPackageHandler.On("VerifyMarker", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.verifyMarkerReturn())
		mockPackageHandler.On("ApplyChanges", mock.Anything, mock.Anything, mock.Anything).Return(tc.applyChangesReturn())
		mockPackageHandler.On("GetReference", mock.Anything, mock.Anything, mock.Anything).Return(tc.getReferenceReturn())
		mockStorageHandler.On("verifyAllocation", mock.Anything, req.Allocation, false).Return(tc.verifyAllocationReturn())
		mockPackageHandler.On("GetWriteMarkerEntity", mock.Anything, mock.Anything).Return(tc.getWriteMarkerEntityReturn())
		mockPackageHandler.On("UpdateAllocationAndCommitChanges", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.updateAllocationAndCommitChangesReturn())

		svc := newGRPCBlobberService(mockStorageHandler, mockPackageHandler)
		resp, err := svc.Commit(ctx, req)
		if err != nil {
			if tc.expectedError {
				continue
			} else {
				t.Fatal("unexpected error - " + err.Error())
			}
		}

		assert.Equal(t, resp.WriteMarker.AllocationID, allocationId)
	}
}
