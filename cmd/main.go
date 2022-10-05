// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	p "plugin-arch-grpc-server-go/cmd/plugin-arch-grpc-server-go"
	"plugin-arch-grpc-server-go/pkg/pb"
)

func main() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := flag.Int("port", 50051, "The server port")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()))

	pb.RegisterMatchFunctionServer(s, &p.MatchFunctionServer{
		UnimplementedMatchFunctionServer: pb.UnimplementedMatchFunctionServer{},
	})
	logrus.Printf("server listening at %v", lis.Addr())

	if err = s.Serve(lis); err != nil {
		logrus.Fatalf("failed to serve: %v", err)
	}
}
