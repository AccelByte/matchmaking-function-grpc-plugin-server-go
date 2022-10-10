// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAppToken(t *testing.T) {
	/*
		url := "https://demo.accelbyte.io/iam/v3/oauth/token"
		data := fmt.Sprintf("grant_type=password&username=%s&password=%s&namespace=%s", "serversdk_user2@dummy.com", "ffd$2e3ebb6fe64cb7b9f9$5486692ad", testNamespace)

		result := struct {
			Token string `json:"access_token"`
		}{}
		authorization := fmt.Sprintf("Basic %s", encodedAdminClientID)
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

	token := GetAppToken("serversdk_user2@dummy.com", "ffd$2e3ebb6fe64cb7b9f9$5486692ad")

	assert.NotNil(t, token)

	/*
		t.Log(resp.StatusCode)
		t.Log(resp.Header.Get("authorization"))
		assert.Nil(t, errs)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	*/
}
