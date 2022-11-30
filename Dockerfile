FROM --platform=$BUILDPLATFORM golang:1.18-alpine as builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum .
RUN go mod download
COPY . .
RUN env GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o plugin-arch-grpc-server-go_$TARGETOS-$TARGETARCH

FROM alpine:3.17.0
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app
COPY --from=builder /build/plugin-arch-grpc-server-go_$TARGETOS-$TARGETARCH plugin-arch-grpc-server-go
# Plugin arch gRPC server port
EXPOSE 6565
# Prometheus /metrics web server port
EXPOSE 8080
CMD [ "/app/plugin-arch-grpc-server-go" ]