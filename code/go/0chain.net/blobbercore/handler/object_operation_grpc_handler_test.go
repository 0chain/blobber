package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/mocks"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"path/filepath"
	"reflect"
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

	req := &blobbergrpc.UpdateObjectAttributesRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     `client`,
			ClientKey:  `client_key`,
			Allocation: `1`,
		},
		Allocation:   `1`,
		Attributes:   `{"who_pays_for_reads" : 1}`,
		Path:         `path`,
		ConnectionId: `connection_id`,
	}
	resOk := &blobbergrpc.UpdateObjectAttributesResponse{WhoPaysForReads: int64(1)}

	mockStorageHandler := &storageHandlerI{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).
		Return(&allocation.Allocation{
			Tx:      req.Context.Allocation,
			ID:      req.Allocation,
			OwnerID: req.Context.Client,
		}, nil)

	mockAllocCollector := &mocks.IAllocationChangeCollector{}
	mockAllocCollector.On(`GetConnectionID`).Return(req.ConnectionId)
	mockAllocCollector.On(`GetAllocationID`).Return(req.Allocation)
	mockAllocCollector.On(`AddChange`, mock.Anything, mock.Anything).Return()
	mockAllocCollector.On(`Save`, mock.Anything).Return(nil)
	mockAllocCollector.On(`TableName`).Return(`allocation_connections`)

	mockReferencePackage := &mocks.PackageHandler{}
	mockReferencePackage.On(`GetAllocationChanges`, mock.Anything,
		req.ConnectionId, req.Context.Allocation, req.Context.Client).Return(mockAllocCollector, nil)

	pathHash := req.Context.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, req.Context.Allocation, req.Path).
		Return(pathHash)

	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, req.Context.Allocation, pathHash).
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
				ctx: context.Background(),
				r:   req,
			},
			wantResponse: resOk,
			wantErr:      false,
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

func Test_blobberGRPCService_CopyObject(t *testing.T) {
	type fields struct {
		storageHandler             StorageHandlerI
		packageHandler             PackageHandler
		UnimplementedBlobberServer blobbergrpc.UnimplementedBlobberServer
	}
	type args struct {
		ctx context.Context
		r   *blobbergrpc.CopyObjectRequest
	}

	req := &blobbergrpc.CopyObjectRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     `client`,
			ClientKey:  `client_key`,
			Allocation: `1`,
		},
		Allocation:   `1`,
		Path:         `path`,
		ConnectionId: `connection_id`,
		Dest:         `dest`,
	}

	mockStorageHandler := &storageHandlerI{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).
		Return(&allocation.Allocation{
			Tx:      req.Context.Allocation,
			ID:      req.Allocation,
			OwnerID: req.Context.Client,
		}, nil)

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
		req.ConnectionId, req.Context.Allocation, req.Context.Client).Return(mockAllocCollector, nil)

	pathHash := req.Context.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, req.Context.Allocation, req.Path).
		Return(pathHash)

	objectRef := &reference.Ref{
		Name:        "test",
		ContentHash: `hash`,
		MerkleRoot:  `root`,
		Size:        1,
	}
	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, req.Context.Allocation, pathHash).
		Return(objectRef, nil)
	newPath := filepath.Join(req.Dest, objectRef.Name)
	mockReferencePackage.On(`GetReference`, mock.Anything, req.Context.Allocation, newPath).
		Return(nil, nil)
	mockReferencePackage.On(`GetReference`, mock.Anything, req.Context.Allocation, req.Dest).
		Return(&reference.Ref{Type: `d`}, nil)

	resOk := &blobbergrpc.CopyObjectResponse{
		Filename:     objectRef.Name,
		Size:         objectRef.Size,
		ContentHash:  objectRef.Hash,
		MerkleRoot:   objectRef.MerkleRoot,
		UploadLength: 0,
		UploadOffset: 0,
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *blobbergrpc.CopyObjectResponse
		wantErr bool
	}{
		{
			name: `OK`,
			fields: fields{
				storageHandler:             mockStorageHandler,
				packageHandler:             mockReferencePackage,
				UnimplementedBlobberServer: blobbergrpc.UnimplementedBlobberServer{},
			},
			args: args{
				ctx: context.Background(),
				r:   req,
			},
			want:    resOk,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &blobberGRPCService{
				storageHandler:             tt.fields.storageHandler,
				packageHandler:             tt.fields.packageHandler,
				UnimplementedBlobberServer: tt.fields.UnimplementedBlobberServer,
			}
			got, err := b.CopyObject(tt.args.ctx, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CopyObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_blobberGRPCService_RenameObject(t *testing.T) {
	type fields struct {
		storageHandler             StorageHandlerI
		packageHandler             PackageHandler
		UnimplementedBlobberServer blobbergrpc.UnimplementedBlobberServer
	}
	type args struct {
		ctx context.Context
		r   *blobbergrpc.RenameObjectRequest
	}

	req := &blobbergrpc.RenameObjectRequest{
		Context: &blobbergrpc.RequestContext{
			Client:     `client`,
			ClientKey:  `client_key`,
			Allocation: `1`,
		},
		Allocation:   `1`,
		Path:         `path`,
		ConnectionId: `connection_id`,
		NewName:      `new_name`,
	}

	mockStorageHandler := &storageHandlerI{}
	mockStorageHandler.On("verifyAllocation", mock.Anything, req.Context.Allocation, false).
		Return(&allocation.Allocation{
			Tx:      req.Context.Allocation,
			ID:      req.Allocation,
			OwnerID: req.Context.Client,
		}, nil)

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
		req.ConnectionId, req.Context.Allocation, req.Context.Client).Return(mockAllocCollector, nil)

	pathHash := req.Context.Allocation + `:` + req.Path
	mockReferencePackage.On(`GetReferenceLookup`, mock.Anything, req.Context.Allocation, req.Path).
		Return(pathHash)

	objectRef := &reference.Ref{
		Name:        "test",
		ContentHash: `hash`,
		MerkleRoot:  `root`,
		Size:        1,
	}
	mockReferencePackage.On(`GetReferenceFromLookupHash`, mock.Anything, req.Context.Allocation, pathHash).
		Return(objectRef, nil)

	resOk := &blobbergrpc.RenameObjectResponse{
		Filename:     req.NewName,
		Size:         objectRef.Size,
		ContentHash:  objectRef.Hash,
		MerkleRoot:   objectRef.MerkleRoot,
		UploadLength: 0,
		UploadOffset: 0,
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *blobbergrpc.RenameObjectResponse
		wantErr bool
	}{
		{
			name: `OK`,
			fields: fields{
				storageHandler:             mockStorageHandler,
				packageHandler:             mockReferencePackage,
				UnimplementedBlobberServer: blobbergrpc.UnimplementedBlobberServer{},
			},
			args: args{
				ctx: context.Background(),
				r:   req,
			},
			want:    resOk,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &blobberGRPCService{
				storageHandler:             tt.fields.storageHandler,
				packageHandler:             tt.fields.packageHandler,
				UnimplementedBlobberServer: tt.fields.UnimplementedBlobberServer,
			}
			got, err := b.RenameObject(tt.args.ctx, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenameObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RenameObject() got = %v, want %v", got, tt.want)
			}
		})
	}
}
