// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/google/uuid"
	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
)

const (
	testNamespace        = "sdktestnamespace"
	encodedAdminClientID = "MDY0MzMxNTk0MDMwNDhkNDhkOWRmNzY2MTIxYjVhOWI6SmZSMkh1RWlDLEF3Z3daZ1djQVlkR0xiY2xCd2QlNEU="
)

func ValidateAuth(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")

	return token == "foo"
}

// GenerateUUID generates uuid without hyphens
func GenerateUUID() string {
	id, _ := uuid.NewRandom()

	return strings.ReplaceAll(id.String(), "-", "")
}

func GetAppToken(username string, password string) string {
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
