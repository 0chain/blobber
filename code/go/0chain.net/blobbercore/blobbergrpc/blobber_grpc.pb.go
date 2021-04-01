// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package blobbergrpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// BlobberClient is the client API for Blobber service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BlobberClient interface {
	GetAllocation(ctx context.Context, in *GetAllocationRequest, opts ...grpc.CallOption) (*GetAllocationResponse, error)
	GetFileMetaData(ctx context.Context, in *GetFileMetaDataRequest, opts ...grpc.CallOption) (*GetFileMetaDataResponse, error)
	GetFileStats(ctx context.Context, in *GetFileStatsRequest, opts ...grpc.CallOption) (*GetFileStatsResponse, error)
}

type blobberClient struct {
	cc grpc.ClientConnInterface
}

func NewBlobberClient(cc grpc.ClientConnInterface) BlobberClient {
	return &blobberClient{cc}
}

func (c *blobberClient) GetAllocation(ctx context.Context, in *GetAllocationRequest, opts ...grpc.CallOption) (*GetAllocationResponse, error) {
	out := new(GetAllocationResponse)
	err := c.cc.Invoke(ctx, "/blobber.service.v1.Blobber/GetAllocation", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blobberClient) GetFileMetaData(ctx context.Context, in *GetFileMetaDataRequest, opts ...grpc.CallOption) (*GetFileMetaDataResponse, error) {
	out := new(GetFileMetaDataResponse)
	err := c.cc.Invoke(ctx, "/blobber.service.v1.Blobber/GetFileMetaData", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *blobberClient) GetFileStats(ctx context.Context, in *GetFileStatsRequest, opts ...grpc.CallOption) (*GetFileStatsResponse, error) {
	out := new(GetFileStatsResponse)
	err := c.cc.Invoke(ctx, "/blobber.service.v1.Blobber/GetFileStats", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BlobberServer is the server API for Blobber service.
// All implementations must embed UnimplementedBlobberServer
// for forward compatibility
type BlobberServer interface {
	GetAllocation(context.Context, *GetAllocationRequest) (*GetAllocationResponse, error)
	GetFileMetaData(context.Context, *GetFileMetaDataRequest) (*GetFileMetaDataResponse, error)
	GetFileStats(context.Context, *GetFileStatsRequest) (*GetFileStatsResponse, error)
	mustEmbedUnimplementedBlobberServer()
}

// UnimplementedBlobberServer must be embedded to have forward compatible implementations.
type UnimplementedBlobberServer struct {
}

func (UnimplementedBlobberServer) GetAllocation(context.Context, *GetAllocationRequest) (*GetAllocationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAllocation not implemented")
}
func (UnimplementedBlobberServer) GetFileMetaData(context.Context, *GetFileMetaDataRequest) (*GetFileMetaDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFileMetaData not implemented")
}
func (UnimplementedBlobberServer) GetFileStats(context.Context, *GetFileStatsRequest) (*GetFileStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFileStats not implemented")
}
func (UnimplementedBlobberServer) mustEmbedUnimplementedBlobberServer() {}

// UnsafeBlobberServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BlobberServer will
// result in compilation errors.
type UnsafeBlobberServer interface {
	mustEmbedUnimplementedBlobberServer()
}

func RegisterBlobberServer(s grpc.ServiceRegistrar, srv BlobberServer) {
	s.RegisterService(&Blobber_ServiceDesc, srv)
}

func _Blobber_GetAllocation_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAllocationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlobberServer).GetAllocation(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blobber.service.v1.Blobber/GetAllocation",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlobberServer).GetAllocation(ctx, req.(*GetAllocationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Blobber_GetFileMetaData_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetFileMetaDataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlobberServer).GetFileMetaData(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blobber.service.v1.Blobber/GetFileMetaData",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlobberServer).GetFileMetaData(ctx, req.(*GetFileMetaDataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Blobber_GetFileStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetFileStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlobberServer).GetFileStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blobber.service.v1.Blobber/GetFileStats",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlobberServer).GetFileStats(ctx, req.(*GetFileStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Blobber_ServiceDesc is the grpc.ServiceDesc for Blobber service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Blobber_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "blobber.service.v1.Blobber",
	HandlerType: (*BlobberServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetAllocation",
			Handler:    _Blobber_GetAllocation_Handler,
		},
		{
			MethodName: "GetFileMetaData",
			Handler:    _Blobber_GetFileMetaData_Handler,
		},
		{
			MethodName: "GetFileStats",
			Handler:    _Blobber_GetFileStats_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "blobber.proto",
}
