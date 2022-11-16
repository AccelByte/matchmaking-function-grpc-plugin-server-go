// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/propagators/b3"
	"google.golang.org/grpc/reflection"
	matchfunctiongrpc "plugin-arch-grpc-server-go/pkg/pb"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"plugin-arch-grpc-server-go/pkg/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	// Create the Tempo exporter
	conn, err := grpc.DialContext(ctx, "tempo:4317", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// metrics
	enableMetrics(ctx, grpcServer)

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tracerProvider, nil
}

func enableMetrics(ctx context.Context, grpcServer *grpc.Server) {
	// The exporter embeds a default OpenTelemetry Reader and
	// implements prometheus.Collector, allowing it to be used as
	// both a Reader and Collector.
	rdr := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(rdr))
	meter := provider.Meter("plugin-arch-grpc-server-instrumentation")

	// Start the prometheus HTTP server and pass the exporter Collector to it
	go serveMetrics(grpcServer)

	// This is the equivalent of prometheus.NewHistogramVec
	histogram, err := meter.SyncFloat64().Histogram("histogram")
	if err != nil {
		logrus.Fatal(err)
	}
	startTime := time.Now()
	histogram.Record(ctx, float64(time.Since(startTime).Milliseconds()))

	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)
	<-ctx.Done()
}

func serveMetrics(grpcServer *grpc.Server) {
	// Create a HTTP server for prometheus.
	reg := prometheus.NewRegistry()
	httpServer := &http.Server{Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), Addr: "0.0.0.0:8080"}
	logrus.Printf("serving metrics at localhost:8080/metrics")

	// Create some standard server metrics.
	grpcMetrics := grpcPrometheus.NewServerMetrics()

	// Initialize all metrics.
	grpcMetrics.InitializeMetrics(grpcServer)

	// Start your http server for prometheus.
	logrus.Printf("listen and serve for prometheus")
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			logrus.Fatalf("Unable to start a http server. %s", err.Error())
		}
	}()

	logrus.Printf("listen and served")
}

func main() {
	logrus.Infof("starting app server.")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	logrus.Infof("listening to grpc port.")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)

		return
	}

	// propagator for envoy
	header := http.Header{}
	propagator := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
	ctx = propagator.Extract(ctx, propagation.HeaderCarrier(header))

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
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			logrus.Fatal(err)
		}
	}(ctx)

	tr := tp.Tracer("server-component-main")

	_, span := tr.Start(ctx, "main")
	span.End()

	flag.Parse()
}
