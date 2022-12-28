// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	"github.com/stretchr/testify/assert"

	sdkAuth "github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
)

func TestTokenValidator_ValidateToken(t *testing.T) {
	// Arrange
	configRepo := sdkAuth.DefaultConfigRepositoryImpl()
	tokenRepo := sdkAuth.DefaultTokenRepositoryImpl()
	authService := iam.OAuth20Service{
		Client:           factory.NewIamClient(configRepo),
		ConfigRepository: configRepo,
		TokenRepository:  tokenRepo,
	}

	err := authService.LoginClient(&configRepo.ClientId, &configRepo.ClientSecret)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	accessToken, err := authService.GetToken()
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	namespace := "accelbyte"
	resourceName := "MMV2GRPCSERVICE"
	requiredPermission := Permission{
		Action:   2,
		Resource: fmt.Sprintf("NAMESPACE:%s:%s", namespace, resourceName),
	}

	validator := NewTokenValidator(authService, time.Hour)
	validator.Initialize()

	// Act
	err = validator.Validate(accessToken, &requiredPermission, &namespace, nil)

	// Assert
	assert.Nil(t, err)
}
