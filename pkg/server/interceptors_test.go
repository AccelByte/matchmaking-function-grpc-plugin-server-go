// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"google.golang.org/grpc"
)

func ExampleStreamClientInterceptor() {
	s := NewGRPCStreamClientInterceptor()
	_, _ = grpc.Dial("localhost", grpc.WithStreamInterceptor(s))
}

func ExampleUnaryClientInterceptor() {
	s := NewGRPUnaryClientInterceptor()
	_, _ = grpc.Dial("localhost", grpc.WithUnaryInterceptor(s))
}

func ExampleStreamServerInterceptor() {
	s := NewGRPCStreamServerInterceptor()
	_ = grpc.NewServer(grpc.StreamInterceptor(s))
}

func ExampleUnaryServerInterceptor() {
	s := NewGRPUnaryServerInterceptor()
	_ = grpc.NewServer(grpc.UnaryInterceptor(s))
}
