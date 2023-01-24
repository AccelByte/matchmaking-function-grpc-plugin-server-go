// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth/validator"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	Validator validator.AuthTokenValidator
)

func UnaryAuthServerIntercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if Validator == nil {
		return nil, errors.New("server token validator not set")
	}

	meta, found := metadata.FromIncomingContext(ctx)
	if !found {
		return nil, errors.New("metadata missing")
	}

	authorization := meta["authorization"][0]
	token := strings.TrimPrefix(authorization, "Bearer ")

	namespace := getNamespace()
	permission := getRequiredPermission()
	var userId *string

	err := Validator.Validate(token, &permission, &namespace, userId)
	if err != nil {
		return nil, err
	}

	return handler(ctx, req)
}

func StreamAuthServerIntercept(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if Validator == nil {
		return errors.New("server token validator not set")
	}

	meta, found := metadata.FromIncomingContext(ss.Context())
	if !found {
		return errors.New("metadata missing")
	}

	authorization := meta["authorization"][0]
	token := strings.TrimPrefix(authorization, "Bearer ")

	namespace := getNamespace()
	permission := getRequiredPermission()
	var userId *string

	err := Validator.Validate(token, &permission, &namespace, userId)
	if err != nil {
		return err
	}

	return handler(srv, ss)
}

func getAction() int {
	return GetEnvInt("AB_ACTION", 2)
}

func getNamespace() string {
	return GetEnv("AB_NAMESPACE", "accelbyte")
}

func getResourceName() string {
	return GetEnv("AB_RESOURCE_NAME", "MMV2GRPCSERVICE")
}

func getRequiredPermission() validator.Permission {
	return validator.Permission{
		Action:   getAction(),
		Resource: fmt.Sprintf("NAMESPACE:%s:%s", getNamespace(), getResourceName()),
	}
}
