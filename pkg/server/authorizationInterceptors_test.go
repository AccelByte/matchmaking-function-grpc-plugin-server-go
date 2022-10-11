// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAppToken(t *testing.T) {
	/*
		url := "https://demo.accelbyte.io/iam/v3/oauth/token"
		data := fmt.Sprintf("grant_type=password&username=%s&password=%s&namespace=%s", os.Getenv("AB_USERNAME"), os.Getenv("AB_PASSWORD"), TestNamespace)

		result := struct {
			Token string `json:"access_token"`
		}{}
		authorization := fmt.Sprintf("Basic %s", EncodedAdminClientID)
		resp, _, errs := gorequest.New().
			Post(url).
			Set("Authorization", authorization).
			Set("Accept", MIME_JSON).
			Type(gorequest.TypeForm).
			Send(data).
			EndStruct(&result)
		if len(errs) != 0 {
			logrus.Error(errs)
			t.Fatal(errs)
		}
	*/

	token := GetToken(os.Getenv("AB_USERNAME"), os.Getenv("AB_PASSWORD"))

	assert.NotNil(t, token)

	/*
		t.Log(resp.StatusCode)
		t.Log(resp.Header.Get("authorization"))
		assert.Nil(t, errs)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	*/
}
