// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"strings"
	"time"

	"github.com/AccelByte/accelbyte-go-sdk/iam-sdk/pkg/iamclient/o_auth2_0"
	"github.com/AccelByte/accelbyte-go-sdk/iam-sdk/pkg/iamclientmodels"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
	"github.com/AccelByte/bloom"
	"github.com/AccelByte/go-jose/jwt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	jwtEncoding  = base64.URLEncoding.WithPadding(base64.NoPadding)
	keys         = make(map[string]*rsa.PublicKey)
	revokedUsers = make(map[string]time.Time)
	jwtClaims    = JWTClaims{}
	configRepo   = *auth.DefaultConfigRepositoryImpl()
	tokenRepo    = *auth.DefaultTokenRepositoryImpl()
	oauthService = &iam.OAuth20Service{
		Client:           factory.NewIamClient(&configRepo),
		ConfigRepository: &configRepo,
		TokenRepository:  &tokenRepo,
	}
)

func ValidateAuth(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")

	return TokenValidator(token)
}

func TokenValidator(accessToken string) bool {
	// store the accessToken
	token := &iamclientmodels.OauthmodelTokenResponseV3{
		AccessToken: &accessToken,
	}
	errStoreToken := oauthService.TokenRepository.Store(token)
	if errStoreToken != nil {
		logrus.Error(errStoreToken)

		return false
	}

	// parse signed
	parsedSignedToken, err := jwt.ParseSigned(accessToken)
	if err != nil {
		logrus.Error("validateJWT: unable to parse JWT. ", err)

		return false
	}
	if parsedSignedToken.Headers[0].KeyID == "" {
		logrus.Error("invalid header when parsed signed.")

		return false
	}

	// fetch jwks key using public key
	errFetchJWKS := fetchJWKS()
	if errFetchJWKS != nil {
		logrus.Error(errFetchJWKS)

		return false
	}
	if keys == nil {
		logrus.Error("public key to validate is nil")

		return false
	}

	// claims
	errClaims := parsedSignedToken.Claims(keys[parsedSignedToken.Headers[0].KeyID], &jwtClaims)
	if errClaims != nil {
		logrus.Error(errClaims)

		return false
	}

	// checks expiration time
	err = jwtClaims.Validate()
	if err != nil {
		if err == jwt.ErrExpired {
			logrus.Error("token expired. ", errClaims)

			return false
		}
		logrus.Error("unable to validate JWT. ", err)

		return false
	}

	// get revocation list
	errRevocationList := getRevocationList(accessToken)
	if errRevocationList != nil {
		logrus.Error(errRevocationList)

		return false
	}

	// check if user is in revocation list
	for userID, revokedAt := range revokedUsers {
		if userID == jwtClaims.Subject {
			logrus.Error("user is found but token is revoked.")

			return false
		}
		if revokedAt.Unix() >= int64(jwtClaims.IssuedAt) {
			logrus.Error("token is revoked because of issued time")

			return false
		}
	}

	// validate token permission
	requiredPermission := Permission{
		Resource: "NAMESPACE:{namespace}:{resource_name}",
		Action:   2,
	}

	permissionResources := make(map[string]string, 0)
	permissionResources["{namespace}"] = jwtClaims.Namespace

	for placeholder, value := range permissionResources {
		requiredPermission.Resource = strings.Replace(requiredPermission.Resource, placeholder, value, 1)
	}

	for _, grantedPermission := range jwtClaims.Permissions {
		grantedAction := grantedPermission.Action
		if resourceAllowed(grantedPermission.Resource, requiredPermission.Resource) &&
			actionAllowed(grantedAction, requiredPermission.Action) {
			return true
		}
	}

	logrus.Info("ValidatePermission: permission allowed to access resource")

	logrus.Info("JWT validated.")

	return true
}

func resourceAllowed(accessPermissionResource string, requiredPermissionResource string) bool {
	requiredPermResSections := strings.Split(requiredPermissionResource, ":")
	requiredPermResSectionLen := len(requiredPermResSections)
	accessPermResSections := strings.Split(accessPermissionResource, ":")
	accessPermResSectionLen := len(accessPermResSections)

	minSectionLen := accessPermResSectionLen
	if minSectionLen > requiredPermResSectionLen {
		minSectionLen = requiredPermResSectionLen
	}

	for i := 0; i < minSectionLen; i++ {
		userSection := accessPermResSections[i]
		requiredSection := requiredPermResSections[i]

		if userSection != requiredSection && userSection != "*" {
			return false
		}
	}

	if accessPermResSectionLen == requiredPermResSectionLen {
		return true
	}

	if accessPermResSectionLen < requiredPermResSectionLen {
		if accessPermResSections[accessPermResSectionLen-1] == "*" {
			if accessPermResSectionLen < 2 {
				return true
			}

			segment := accessPermResSections[accessPermResSectionLen-2]
			if segment == "NAMESPACE" || segment == "USER" {
				return false
			}

			return true
		}

		return false
	}

	for i := requiredPermResSectionLen; i < accessPermResSectionLen; i++ {
		if accessPermResSections[i] != "*" {
			return false
		}
	}

	return true
}

func actionAllowed(grantedAction int, requiredAction int) bool {
	return grantedAction&requiredAction == requiredAction
}

func fetchJWKS() error {
	input := &o_auth2_0.GetJWKSV3Params{}
	getJWKSV3, err := oauthService.GetJWKSV3Short(input)
	if err != nil {
		logrus.Error(err)

		return nil
	}

	for _, key := range getJWKSV3.Keys {
		publicKey, errGenerate := generatePublicKey(key)
		if errGenerate != nil {
			logrus.Error(errGenerate)

			return errGenerate
		}
		keys[key.Kid] = publicKey
	}

	return nil
}

func generatePublicKey(jwk *iamclientmodels.OauthcommonJWKKey) (*rsa.PublicKey, error) {
	n, err := getModulus(jwk.N)
	if err != nil {
		return nil, err
	}

	e, err := getPublicExponent(jwk.E)
	if err != nil {
		return nil, err
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func getModulus(jwkN string) (*big.Int, error) {
	decodedN, err := jwtEncoding.DecodeString(jwkN)
	if err != nil {
		return nil, errors.Wrap(err, "getModulus: unable to decode JWK modulus string")
	}

	n := big.NewInt(0)
	n.SetBytes(decodedN)

	return n, nil
}

func getPublicExponent(jwkE string) (int, error) {
	decodedE, err := jwtEncoding.DecodeString(jwkE)
	if err != nil {
		return 0, errors.Wrap(err, "getPublicExponent: unable to decode JWK exponent string")
	}

	var eBytes []byte
	if len(eBytes) < 8 {
		eBytes = make([]byte, 8-len(decodedE), 8)
		eBytes = append(eBytes, decodedE...)
	} else {
		eBytes = decodedE
	}

	eReader := bytes.NewReader(eBytes)

	var e uint64

	err = binary.Read(eReader, binary.BigEndian, &e)
	if err != nil {
		return 0, errors.Wrap(err, "getPublicExponent: unable to read JWK exponent bytes")
	}

	return int(e), nil
}

func getRevocationList(accessToken string) error {
	input := &o_auth2_0.GetRevocationListV3Params{}
	revocationList, err := oauthService.GetRevocationListV3Short(input)
	if err != nil {
		return err
	}

	// revoked token
	filter := bloom.From(revocationList.RevokedTokens.Bits, uint(*revocationList.RevokedTokens.K))
	filter.MightContain([]byte(accessToken))

	// revoked user
	for _, revokedUser := range revocationList.RevokedUsers {
		revokedUsers[*revokedUser.ID] = time.Time(revokedUser.RevokedAt)
	}

	return nil
}

// GenerateUUID generates uuid without hyphens
func GenerateUUID() string {
	id, _ := uuid.NewRandom()

	return strings.ReplaceAll(id.String(), "-", "")
}
