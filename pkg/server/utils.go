// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// GenerateUUID generates uuid without hyphens
func GenerateUUID() string {
	id, _ := uuid.NewRandom()

	return strings.ReplaceAll(id.String(), "-", "")
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func GetEnvInt(key string, fallback int) int {
	str := GetEnv(key, strconv.Itoa(fallback))
	val, err := strconv.Atoi(str)
	if err != nil {
		return fallback
	}

	return val
}

// logJSONFormatter is printing the data in log
func logJSONFormatter(data interface{}) string {
	response, err := json.Marshal(data)
	if err != nil {
		logrus.Errorf("failed to marshal json.")

		return ""
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{PrettyPrint: true})

		return string(response)
	}
}

func CheckDiff(aList, bList []string) (diffInA, diffInB []string) {
	aMap := convertStringSliceToMap(aList)
	bMap := convertStringSliceToMap(bList)
	return compareDiffOfSlice(aList, bMap), compareDiffOfSlice(bList, aMap)
}

func convertStringSliceToMap(s []string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for _, v := range s {
		m[v] = struct{}{}
	}
	return m
}

func compareDiffOfSlice(s []string, m map[string]struct{}) (diff []string) {
	for _, v := range s {
		_, exist := m[v]
		if exist {
			continue
		}
		diff = append(diff, v)
	}
	return diff
}
