// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var OAuth *iam.OAuth20Service

func UnaryAuthServerIntercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if OAuth == nil {
		return nil, errors.New("server token validator not set")
	}

	meta, found := metadata.FromIncomingContext(ctx)
	if !found {
		return nil, errors.New("metadata is missing")
	}

	if meta["authorization"] != nil {
		authorization := meta["authorization"][0]
		token := strings.TrimPrefix(authorization, "Bearer ")
		extendNamespace := os.Getenv("AB_NAMESPACE")

		err := OAuth.Validate(token, nil, &extendNamespace, nil)
		if err != nil {
			return nil, err
		}

		fmt.Print("server: token validated. \n")
	}

	return handler(ctx, req)
}

func StreamAuthServerIntercept(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if OAuth == nil {
		return errors.New("server token validator not set")
	}

	meta, found := metadata.FromIncomingContext(ss.Context())
	if !found {
		return errors.New("metadata is missing")
	}

	if meta["authorization"] != nil {
		authorization := meta["authorization"][0]
		token := strings.TrimPrefix(authorization, "Bearer ")
		extendNamespace := os.Getenv("AB_NAMESPACE")

		err := OAuth.Validate(token, nil, &extendNamespace, nil)
		if err != nil {
			return err
		}

		fmt.Print("server: token validated. \n")
	}

	return handler(srv, ss)
}
