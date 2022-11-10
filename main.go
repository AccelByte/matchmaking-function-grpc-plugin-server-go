// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	matchfunctiongrpc "plugin-arch-grpc-server-go/pkg/pb"
	"plugin-arch-grpc-server-go/pkg/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	service     = "custommatchmakingfunctiongoserver"
	environment = "production"
	id          = 1
)

func tracerProvider() (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		)),
	)

	return tp, nil
}

func main() {
	tp, err := tracerProvider()
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	tr := tp.Tracer("server-component-main")

	_, span := tr.Start(ctx, "main")
	span.End()

	flag.Parse()

	port := flag.Int("port", 5500, "The server port")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			server.EnsureValidToken,
		),
		grpc.ChainStreamInterceptor(
			otelgrpc.StreamServerInterceptor(),
		),
	}

	s := grpc.NewServer(opts...)

	matchMaker := server.New()

	matchfunctiongrpc.RegisterMatchFunctionServer(s, &server.MatchFunctionServer{
		MatchMaker: matchMaker,
	})
	reflection.Register(s) // self documentation for the server
	logrus.Printf("gRPC server listening at %v", lis.Addr())

	if err = s.Serve(lis); err != nil {
		logrus.Fatalf("failed to serve: %v", err)
	}
}
