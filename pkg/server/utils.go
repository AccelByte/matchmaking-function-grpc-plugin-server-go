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

// LogJSONFormatter is printing the data in log
func LogJSONFormatter(data interface{}) string {
	response, err := json.Marshal(data)
	if err != nil {
		logrus.Errorf("failed to marshal json.")

		return ""
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{PrettyPrint: true})

		return string(response)
	}
}
