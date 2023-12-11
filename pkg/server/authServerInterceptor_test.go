// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	sdkAuth "github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

type MyConfigRepo struct {
	baseUrl      string
	clientId     string
	clientSecret string
}

func (c *MyConfigRepo) GetClientId() string       { return c.clientId }
func (c *MyConfigRepo) GetClientSecret() string   { return c.clientSecret }
func (c *MyConfigRepo) GetJusticeBaseUrl() string { return c.baseUrl }

func TestTokenValidator_ValidateToken(t *testing.T) {
	t.Skip() // "TODO: mock and remove hardcoded client id and secret"

	// Arrange
	baseUrl := "https://development.accelbyte.io"
	clientId := ""
	clientSecret := ""
	configRepo := &MyConfigRepo{
		baseUrl:      baseUrl,
		clientId:     clientId,
		clientSecret: clientSecret,
	}
	tokenRepo := sdkAuth.DefaultTokenRepositoryImpl()
	authService := iam.OAuth20Service{
		Client:           factory.NewIamClient(configRepo),
		ConfigRepository: configRepo,
		TokenRepository:  tokenRepo,
	}

	err := authService.LoginClient(&clientId, &clientSecret)
	if err != nil {
		assert.Fail(t, err.Error())

		return
	}

	accessToken, err := authService.GetToken()
	if err != nil {
		assert.Fail(t, err.Error())

		return
	}

	authService.SetLocalValidation(true)                                          // true will do it locally, false will do it remotely
	claims, errClaims := authService.ParseAccessTokenToClaims(accessToken, false) // false will not validate using client namespace
	if errClaims != nil {
		assert.Fail(t, errClaims.Error())

		return
	}

	// Act
	err = authService.Validate(accessToken, nil, &claims.ExtendNamespace, nil)

	// Assert
	assert.Nil(t, err)
}
