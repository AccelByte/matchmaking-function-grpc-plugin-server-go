// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/google/uuid"
	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
)

func ValidateAuth(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")

	return token == "some-secret-token"
}

// GenerateUUID generates uuid without hyphens
func GenerateUUID() string {
	id, _ := uuid.NewRandom()

	return strings.ReplaceAll(id.String(), "-", "")
}

func GetToken(username string, password string) string {
	testNamespace := os.Getenv("AB_NAMESPACE")
	encodedAdminClientID := os.Getenv("ENCODED_CLIENT_ID")
	url := "https://demo.accelbyte.io/iam/v3/oauth/token"
	data := fmt.Sprintf("grant_type=password&username=%s&password=%s&namespace=%s", username, password, testNamespace)

	result := struct {
		Token string `json:"access_token"`
	}{}
	authorization := fmt.Sprintf("Basic %s", encodedAdminClientID)
	resp, _, errs := gorequest.New().
		Post(url).
		Set("Authorization", authorization).
		Set("Accept", restful.MIME_JSON).
		Type(gorequest.TypeForm).
		Send(data).
		EndStruct(&result)
	if len(errs) != 0 {
		logrus.Error(errs)

		return ""
	}
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	return result.Token
}
