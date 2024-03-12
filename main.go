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
	_ "net/http/pprof"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	sdkAuth "github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
	promgrpc "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"matchmaking-function-grpc-plugin-server-go/pkg/common"
	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"
	"matchmaking-function-grpc-plugin-server-go/pkg/server"
)

const (
	environment = "production"
	id          = 1
)

var (
	port        = flag.Int("port", 6565, "The grpc server port")
	serviceName = common.GetEnv("OTEL_SERVICE_NAME", "MatchmakingFunctionGrpcPluginServerGoDocker")
)

func main() {
	configureLogging()

	go func() {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(10)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scope := common.NewRootScope(ctx, serviceName, common.GenerateUUID())
	defer scope.Finish()

	scope.Log.Infof("starting app server.")

	srvMetrics := promgrpc.NewServerMetrics()
	unaryServerInterceptors := []grpc.UnaryServerInterceptor{
		otelgrpc.UnaryServerInterceptor(),
		srvMetrics.UnaryServerInterceptor(),
	}
	streamServerInterceptors := []grpc.StreamServerInterceptor{
		otelgrpc.StreamServerInterceptor(),
		srvMetrics.StreamServerInterceptor(),
	}

	if strings.ToLower(common.GetEnv("PLUGIN_GRPC_SERVER_AUTH_ENABLED", "false")) == "true" {
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
		scope.Log.Infof("added auth interceptors")
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
	scope.Log.Infof("gRPC reflection enabled")

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
		scope.Log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	scope.Log.Printf("prometheus metrics served at :8080/metrics")

	scope.Log.Infof("listening to grpc port.")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		scope.Log.Fatalf("failed to listen: %v", err)

		return
	}

	scope.Log.Printf("gRPC server listening at %v", lis.Addr())

	scope.Log.Infof("listening...")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			scope.Log.Fatalf("failed to serve: %v", err)

			return
		}
	}()

	scope.Log.Infof("starting init provider.")

	// Save Tracer Provider
	tracerProvider, err := common.NewTracerProvider(serviceName, environment, id)
	if err != nil {
		scope.Log.Fatalf("failed to create tracer provider: %v", err)

		return
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tracerProvider)
	// Register the B3 propagator globally.
	p := b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		p,
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			scope.Log.Fatal(err)
		}
	}(ctx)

	flag.Parse()

	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	<-ctx.Done()
	scope.Log.Println("Goodbye...")
}

func configureLogging() logrus.Level {
	logLevel, err := logrus.ParseLevel(common.GetEnv("LOG_LEVEL", logrus.InfoLevel.String()))
	if err != nil {
		logrus.Error("unable to parse log level: ", err)
	}
	logrus.SetLevel(logLevel)
	logrus.SetReportCaller(true)

	return logLevel
}
