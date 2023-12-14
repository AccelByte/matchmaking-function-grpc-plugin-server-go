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
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	sdkAuth "github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
	promgrpc "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"
	"matchmaking-function-grpc-plugin-server-go/pkg/server"

	"google.golang.org/grpc"
)

const (
	environment = "production"
	id          = 1
)

var (
	port        = flag.Int("port", 6565, "The grpc server port")
	logLevelStr = server.GetEnv("LOG_LEVEL", logrus.InfoLevel.String())
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

	res := resource.NewWithAttributes(
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

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tracerProvider, nil
}

func main() {
	go func() {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(10)
	}()

	logrus.Infof("starting app server.")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrusLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		logrusLevel = logrus.InfoLevel
	}
	logrusLogger := logrus.New()
	logrusLogger.SetLevel(logrusLevel)

	loggingOptions := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall, logging.PayloadReceived, logging.PayloadSent),
		logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
			if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
				return logging.Fields{"traceID", span.TraceID().String()}
			}

			return nil
		}),
		logging.WithLevels(logging.DefaultClientCodeToLevel),
		logging.WithDurationField(logging.DurationToDurationField),
	}
	srvMetrics := promgrpc.NewServerMetrics()
	unaryServerInterceptors := []grpc.UnaryServerInterceptor{
		otelgrpc.UnaryServerInterceptor(),
		srvMetrics.UnaryServerInterceptor(),
		logging.UnaryServerInterceptor(server.InterceptorLogger(logrusLogger), loggingOptions...),
	}
	streamServerInterceptors := []grpc.StreamServerInterceptor{
		otelgrpc.StreamServerInterceptor(),
		srvMetrics.StreamServerInterceptor(),
		logging.StreamServerInterceptor(server.InterceptorLogger(logrusLogger), loggingOptions...),
	}

	if strings.ToLower(server.GetEnv("PLUGIN_GRPC_SERVER_AUTH_ENABLED", "false")) == "true" {
		configRepo := sdkAuth.DefaultConfigRepositoryImpl()
		tokenRepo := sdkAuth.DefaultTokenRepositoryImpl()
		server.OAuth = &iam.OAuth20Service{
			Client:           factory.NewIamClient(configRepo),
			ConfigRepository: configRepo,
			TokenRepository:  tokenRepo,
		}

		server.OAuth.SetLocalValidation(true)

		unaryServerInterceptors = append(unaryServerInterceptors, server.UnaryAuthServerIntercept)
		streamServerInterceptors = append(streamServerInterceptors, server.StreamAuthServerIntercept)
		logrus.Infof("added auth interceptors")
	}

	// Create gRPC Server
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unaryServerInterceptors...),
		grpc.ChainStreamInterceptor(streamServerInterceptors...),
	)

	matchMaker := server.New()
	matchfunctiongrpc.RegisterMatchFunctionServer(grpcServer, &server.MatchFunctionServer{
		UnimplementedMatchFunctionServer: matchfunctiongrpc.UnimplementedMatchFunctionServer{},
		MM:                               matchMaker,
	})

	// Enable gRPC Reflection
	reflection.Register(grpcServer)
	logrus.Infof("gRPC reflection enabled")

	// Enable gRPC Health Check
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())

	// Add go runtime metrics and process collectors.
	srvMetrics.InitializeMetrics(grpcServer)
	promRegistry := prometheus.NewRegistry()
	promRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		srvMetrics,
	)

	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	logrus.Printf("prometheus metrics served at :8080/metrics")

	logrus.Infof("listening to grpc port.")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)

		return
	}

	logrus.Printf("gRPC server listening at %v", lis.Addr())

	logrus.Infof("listening...")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			logrus.Fatalf("failed to serve: %v", err)

			return
		}
	}()

	logrus.Infof("starting init provider.")
	tp, err := initProvider(ctx, grpcServer)
	if err != nil {
		logrus.Fatalf("failed to initializing the provider. %s", err.Error())

		return
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)
	// Register the B3 propagator globally.
	p := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		p,
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			logrus.Fatal(err)
		}
	}(ctx)

	flag.Parse()

	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	logrus.Println("Goodbye...")
}
