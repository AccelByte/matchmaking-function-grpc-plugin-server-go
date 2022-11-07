// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func Test_GetAppToken(t *testing.T) {
	// prepare server
	authorization := map[string]string{"authorization": "Bearer foo"}
	optsServer := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			EnsureValidTokenTest,
		),
		grpc.ChainStreamInterceptor(
			otelgrpc.StreamServerInterceptor(),
		),
	}
	s := grpc.NewServer(optsServer...)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// prepare client
	optsClient := []grpc.DialOption{
		grpc.WithUnaryInterceptor(
			otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(
			otelgrpc.StreamClientInterceptor()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// act
	conn, err := grpc.Dial("0", optsClient...)
	if err != nil {
		logrus.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	md := metadata.New(authorization)

	// assert
	assert.NotNil(t, s)
	assert.NotNil(t, md)
	assert.NotNil(t, md["authorization"])
	assert.Equal(t, md["authorization"][0], authorization["authorization"])
}

func EnsureValidTokenTest(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// The keys within metadata.MD are normalized to lowercase.
	// See: https://godoc.org/google.golang.org/grpc/metadata#New
	if !ValidateAuth([]string{"Bearer foo"}) {
		return nil, fmt.Errorf("error invalid token")
	}
	// Continue execution of handler after ensuring a valid token.
	return handler(ctx, req)
}
