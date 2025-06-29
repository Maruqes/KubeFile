// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.19.6
// source: proto/shortener.proto

package shortener

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	Shortener_ShortURL_FullMethodName   = "/shortener.Shortener/ShortURL"
	Shortener_ResolveURL_FullMethodName = "/shortener.Shortener/ResolveURL"
)

// ShortenerClient is the client API for Shortener service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ShortenerClient interface {
	ShortURL(ctx context.Context, in *ShortURLRequest, opts ...grpc.CallOption) (*ShortURLResponse, error)
	ResolveURL(ctx context.Context, in *ResolveURLRequest, opts ...grpc.CallOption) (*ResolveURLResponse, error)
}

type shortenerClient struct {
	cc grpc.ClientConnInterface
}

func NewShortenerClient(cc grpc.ClientConnInterface) ShortenerClient {
	return &shortenerClient{cc}
}

func (c *shortenerClient) ShortURL(ctx context.Context, in *ShortURLRequest, opts ...grpc.CallOption) (*ShortURLResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ShortURLResponse)
	err := c.cc.Invoke(ctx, Shortener_ShortURL_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *shortenerClient) ResolveURL(ctx context.Context, in *ResolveURLRequest, opts ...grpc.CallOption) (*ResolveURLResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ResolveURLResponse)
	err := c.cc.Invoke(ctx, Shortener_ResolveURL_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ShortenerServer is the server API for Shortener service.
// All implementations must embed UnimplementedShortenerServer
// for forward compatibility
type ShortenerServer interface {
	ShortURL(context.Context, *ShortURLRequest) (*ShortURLResponse, error)
	ResolveURL(context.Context, *ResolveURLRequest) (*ResolveURLResponse, error)
	mustEmbedUnimplementedShortenerServer()
}

// UnimplementedShortenerServer must be embedded to have forward compatible implementations.
type UnimplementedShortenerServer struct {
}

func (UnimplementedShortenerServer) ShortURL(context.Context, *ShortURLRequest) (*ShortURLResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ShortURL not implemented")
}
func (UnimplementedShortenerServer) ResolveURL(context.Context, *ResolveURLRequest) (*ResolveURLResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ResolveURL not implemented")
}
func (UnimplementedShortenerServer) mustEmbedUnimplementedShortenerServer() {}

// UnsafeShortenerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ShortenerServer will
// result in compilation errors.
type UnsafeShortenerServer interface {
	mustEmbedUnimplementedShortenerServer()
}

func RegisterShortenerServer(s grpc.ServiceRegistrar, srv ShortenerServer) {
	s.RegisterService(&Shortener_ServiceDesc, srv)
}

func _Shortener_ShortURL_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShortURLRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShortenerServer).ShortURL(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Shortener_ShortURL_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShortenerServer).ShortURL(ctx, req.(*ShortURLRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Shortener_ResolveURL_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ResolveURLRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShortenerServer).ResolveURL(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Shortener_ResolveURL_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShortenerServer).ResolveURL(ctx, req.(*ResolveURLRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Shortener_ServiceDesc is the grpc.ServiceDesc for Shortener service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Shortener_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "shortener.Shortener",
	HandlerType: (*ShortenerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ShortURL",
			Handler:    _Shortener_ShortURL_Handler,
		},
		{
			MethodName: "ResolveURL",
			Handler:    _Shortener_ResolveURL_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/shortener.proto",
}
