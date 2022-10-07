# Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
# This is licensed software from AccelByte Inc, for limitations
# and restrictions contact your company contract manager.

SHELL := /bin/bash

GOLANG_DOCKER_IMAGE := golang:1.16

.PHONY: build test

build:
	docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ -e GOCACHE=/data/.cache/go-build $(GOLANG_DOCKER_IMAGE) \
		sh -c "go run cmd/main.go"

test:
	docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ -e GOCACHE=/data/.cache/go-build $(GOLANG_DOCKER_IMAGE) \
		sh -c "go test plugin-arch-grpc-server-go/cmd/plugin-arch-grpc-server-go"

proto:
	protoc --proto_path=pkg/proto --go_out=pkg/pb \
	--go_opt=paths=source_relative --go-grpc_out=pkg/pb \
	--go-grpc_opt=paths=source_relative pkg/proto/*.proto
