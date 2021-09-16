// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package adminserver

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

// AdminServerClient is the client API for AdminServer service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AdminServerClient interface {
	DeleteFile(ctx context.Context, in *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error)
	CleanTombstones(ctx context.Context, in *CleanTombstonesRequest, opts ...grpc.CallOption) (*CleanTombstonesResponse, error)
}

type adminServerClient struct {
	cc grpc.ClientConnInterface
}

func NewAdminServerClient(cc grpc.ClientConnInterface) AdminServerClient {
	return &adminServerClient{cc}
}

func (c *adminServerClient) DeleteFile(ctx context.Context, in *DeleteFileRequest, opts ...grpc.CallOption) (*DeleteFileResponse, error) {
	out := new(DeleteFileResponse)
	err := c.cc.Invoke(ctx, "/adminserver.AdminServer/DeleteFile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *adminServerClient) CleanTombstones(ctx context.Context, in *CleanTombstonesRequest, opts ...grpc.CallOption) (*CleanTombstonesResponse, error) {
	out := new(CleanTombstonesResponse)
	err := c.cc.Invoke(ctx, "/adminserver.AdminServer/CleanTombstones", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AdminServerServer is the server API for AdminServer service.
// All implementations must embed UnimplementedAdminServerServer
// for forward compatibility
type AdminServerServer interface {
	DeleteFile(context.Context, *DeleteFileRequest) (*DeleteFileResponse, error)
	CleanTombstones(context.Context, *CleanTombstonesRequest) (*CleanTombstonesResponse, error)
	mustEmbedUnimplementedAdminServerServer()
}

// UnimplementedAdminServerServer must be embedded to have forward compatible implementations.
type UnimplementedAdminServerServer struct {
}

func (UnimplementedAdminServerServer) DeleteFile(context.Context, *DeleteFileRequest) (*DeleteFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteFile not implemented")
}
func (UnimplementedAdminServerServer) CleanTombstones(context.Context, *CleanTombstonesRequest) (*CleanTombstonesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CleanTombstones not implemented")
}
func (UnimplementedAdminServerServer) mustEmbedUnimplementedAdminServerServer() {}

// UnsafeAdminServerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AdminServerServer will
// result in compilation errors.
type UnsafeAdminServerServer interface {
	mustEmbedUnimplementedAdminServerServer()
}

func RegisterAdminServerServer(s grpc.ServiceRegistrar, srv AdminServerServer) {
	s.RegisterService(&AdminServer_ServiceDesc, srv)
}

func _AdminServer_DeleteFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteFileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AdminServerServer).DeleteFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/adminserver.AdminServer/DeleteFile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AdminServerServer).DeleteFile(ctx, req.(*DeleteFileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AdminServer_CleanTombstones_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CleanTombstonesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AdminServerServer).CleanTombstones(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/adminserver.AdminServer/CleanTombstones",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AdminServerServer).CleanTombstones(ctx, req.(*CleanTombstonesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AdminServer_ServiceDesc is the grpc.ServiceDesc for AdminServer service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AdminServer_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "adminserver.AdminServer",
	HandlerType: (*AdminServerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "DeleteFile",
			Handler:    _AdminServer_DeleteFile_Handler,
		},
		{
			MethodName: "CleanTombstones",
			Handler:    _AdminServer_CleanTombstones_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "adminserver/adminserver.proto",
}