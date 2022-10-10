// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func ExampleStreamClientInterceptor() {
	_, _ = grpc.Dial("localhost", grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
}

func ExampleUnaryClientInterceptor() {
	_, _ = grpc.Dial("localhost", grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()))
}

func ExampleStreamServerInterceptor() {
	_ = grpc.NewServer(grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()))
}

func ExampleUnaryServerInterceptor() {
	_ = grpc.NewServer(grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()))
}
