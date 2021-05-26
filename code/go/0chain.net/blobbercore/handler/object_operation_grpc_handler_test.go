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

func Test_blobberGRPCService_UpdateObjectAttributes(t *testing.T) {
	type fields struct {
		storageHandler             StorageHandlerI
		packageHandler             PackageHandler
		UnimplementedBlobberServer blobbergrpc.UnimplementedBlobberServer
	}
	type args struct {
		ctx context.Context
		r   *blobbergrpc.UpdateObjectAttributesRequest
	}

	allocationTx := randString(32)
	pubKey, _, signScheme := GeneratePubPrivateKey(t)
	clientSignature, _ := signScheme.Sign(encryption.Hash(allocationTx))
	req := &blobbergrpc.UpdateObjectAttributesRequest{
		Allocation:   allocationTx,
		Attributes:   `{"who_pays_for_reads" : 1}`,
		Path:         `path`,
		ConnectionId: `connection_id`,
	}

	reqInvalid := &blobbergrpc.UpdateObjectAttributesRequest{
		Allocation:   `invalid_id`,
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
	mockStorageHandler.On("verifyAllocation", mock.Anything, reqInvalid.Allocation, false).
		Return(nil, errors.New("some error"))

	mockAllocCollector := &mocks.IAllocationChangeCollector{}
	mockAllocCollector.On(`GetConnectionID`).Return(req.ConnectionId)
	mockAllocCollector.On(`GetAllocationID`).Return(req.Allocation)
	mockAllocCollector.On(`AddChange`, mock.Anything, mock.Anything).Return()
	mockAllocCollector.On(`Save`, mock.Anything).Return(nil)
	mockAllocCollector.On(`TableName`).Return(`allocation_connections`)

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, alloc.ID, `client`).Return(mockAllocCollector, nil)

	pathHash := req.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, alloc.ID, req.Path).
		Return(pathHash)

	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, alloc.ID, pathHash).
		Return(
			&reference.Ref{
				Name: "test",
				Type: reference.FILE,
			}, nil)

	tests := []struct {
		name         string
		fields       fields
		args         args
		wantResponse *blobbergrpc.UpdateObjectAttributesResponse
		wantErr      bool
	}{
		{
			name: `OK`,
			fields: fields{
				storageHandler: mockStorageHandler,
				packageHandler: mockReferencePackage,
			},
			args: args{
				ctx: ctx,
				r:   req,
			},
			wantResponse: resOk,
			wantErr:      false,
		},
		{
			name: `Invalid_Allocation`,
			fields: fields{
				storageHandler: mockStorageHandler,
				packageHandler: mockReferencePackage,
			},
			args: args{
				ctx: ctx,
				r:   reqInvalid,
			},
			wantResponse: nil,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &blobberGRPCService{
				storageHandler:             tt.fields.storageHandler,
				packageHandler:             tt.fields.packageHandler,
				UnimplementedBlobberServer: tt.fields.UnimplementedBlobberServer,
			}
			gotResponse, err := b.UpdateObjectAttributes(tt.args.ctx, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateObjectAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, gotResponse, tt.wantResponse) {
				t.Errorf("UpdateObjectAttributes() gotResponse = %v, want %v", gotResponse, tt.wantResponse)
			}
		})
	}
}
