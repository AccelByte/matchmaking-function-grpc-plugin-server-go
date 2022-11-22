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
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	matchfunctiongrpc "plugin-arch-grpc-server-go/pkg/pb"

	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/reflection"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"plugin-arch-grpc-server-go/pkg/server"

	"google.golang.org/grpc"
)

const (
	environment = "production"
	id          = 1
)

var (
	service = os.Getenv("OTEL_SERVICE_NAME")
	port    = flag.Int("port", 6565, "The grpc server port")
	res     = resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(service),
		attribute.String("environment", environment),
		attribute.Int64("ID", id),
	)
)

func initProvider(ctx context.Context, grpcServer *grpc.Server) (*sdktrace.TracerProvider, error) {
	// Create Zipkin Exporter and install it as a global tracer.
	//
	// For demoing purposes, always sample. In a production application, you should
	// configure the sampler to a trace.ParentBased(trace.TraceIDRatioBased) set at the desired
	// ratio.
	exporter, err := zipkin.New(os.Getenv("OTEL_EXPORTER_ZIPKIN_ENDPOINT"))
	if err != nil {
		logrus.Fatalf("failed to call zipkin exporter. %s", err.Error())
	}

	res = resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(os.Getenv("OTEL_SERVICE_NAME")),
		attribute.String("environment", environment),
		attribute.Int64("ID", id),
	)

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second*1)),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	//otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tracerProvider, nil
}

func main() {
	logrus.Infof("starting app server.")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			grpcPrometheus.UnaryServerInterceptor,
			server.EnsureValidToken,
		),
		grpc.ChainStreamInterceptor(
			otelgrpc.StreamServerInterceptor(),
			grpcPrometheus.StreamServerInterceptor,
		),
	}

	s := grpc.NewServer(opts...)

	// prometheus metrics
	grpcPrometheus.Register(s)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	logrus.Printf("prometheus metrics served at :8080/metrics")

	logrus.Infof("listening to grpc port.")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)

		return
	}

	p := b3.New()
	// Register the B3 propagator globally.
	otel.SetTextMapPropagator(p)

	// cal the server grpc
	matchMaker := server.New()

	matchfunctiongrpc.RegisterMatchFunctionServer(s, &server.MatchFunctionServer{
		MatchMaker: matchMaker,
	})
	logrus.Infof("adding the grpc reflection.")
	reflection.Register(s) // self documentation for the server
	logrus.Printf("gRPC server listening at %v", lis.Addr())

	logrus.Infof("listening...")
	go func() {
		if err = s.Serve(lis); err != nil {
			logrus.Fatalf("failed to serve: %v", err)

			return
		}
	}()

	logrus.Infof("starting init provider.")
	tp, err := initProvider(ctx, s)
	if err != nil {
		logrus.Fatalf("failed to initializing the provider. %s", err.Error())

		return
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)
	// otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
	// 	propagation.TraceContext{},
	// 	propagation.Baggage{},
	// ))

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			logrus.Fatal(err)
		}
	}(ctx)

	tr := tp.Tracer("server-component-main")

	_, span := tr.Start(ctx, "main")
	span.End()

	flag.Parse()

	// Create some standard server metrics.
	grpcMetrics := grpcPrometheus.NewServerMetrics()

	// Initialize all metrics.
	grpcMetrics.InitializeMetrics(s)
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)
	<-ctx.Done()
}
