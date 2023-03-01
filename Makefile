# Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
# This is licensed software from AccelByte Inc, for limitations
# and restrictions contact your company contract manager.

SHELL := /bin/bash

GOLANG_DOCKER_IMAGE := golang:1.18
IMAGE_NAME ?= $(shell basename "$$(pwd)")-app
IMAGE_VERSION ?= latest

proto:
	rm -rfv pkg/pb/*.pb.go
	mkdir -p pkg/pb
	docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ rvolosatovs/protoc:4.0.0 \
			--proto_path=pkg/proto --go_out=pkg/pb \
			--go_opt=paths=source_relative --go-grpc_out=pkg/pb \
			--go-grpc_opt=paths=source_relative pkg/proto/*.proto

lint: proto
	rm -f lint.err
	find -type f -iname go.mod -exec dirname {} \; | while read DIRECTORY; do \
		echo "# $$DIRECTORY"; \
		docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ -e GOCACHE=/data/.cache/go-build -e GOLANGCI_LINT_CACHE=/data/.cache/go-lint golangci/golangci-lint:v1.42.1\
				sh -c "cd $$DIRECTORY && golangci-lint -v --timeout 5m --max-same-issues 0 --max-issues-per-linter 0 --color never run || touch /data/lint.err"; \
	done
	[ ! -f lint.err ] || (rm lint.err && exit 1)

build: proto
	docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ -e GOCACHE=/data/.cache/go-build $(GOLANG_DOCKER_IMAGE) \
		sh -c "go build"

image:
	docker buildx build -t ${IMAGE_NAME} --load .

imagex:
	docker buildx inspect my-builder \
			|| docker buildx create --name my-builder --use
	docker buildx build -t ${IMAGE_NAME}:${IMAGE_VERSION} --platform linux/arm64/v8,linux/amd64 .
	docker buildx build -t ${IMAGE_NAME}:${IMAGE_VERSION} --load .
	#docker buildx rm my-builder

imagex-push:
	docker buildx inspect my-builder-2 \
			|| docker buildx create --name my-builder-2 --use
	docker buildx build -t ${IMAGE_NAME}:${IMAGE_VERSION} --platform linux/amd64 --push .
	#docker buildx rm my-builder-2

test: proto
	docker run -t --rm -u $$(id -u):$$(id -g) -v $$(pwd):/data/ -w /data/ -e GOCACHE=/data/.cache/go-build $(GOLANG_DOCKER_IMAGE) \
		sh -c "go test matchmaking-function-grpc-plugin-server-go/pkg/server"


