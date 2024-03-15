// Copyright (c) 2024 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sirupsen/logrus"
)

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

// GenerateRandomInt generate a random int that is not determined
func GenerateRandomInt() int {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	return random.Intn(10000)
}

// MakeTraceID create new traceID
// example: service_1234
func MakeTraceID(identifiers ...string) string {
	strInt := strconv.Itoa(GenerateRandomInt())
	var tID string
	for _, i := range identifiers {
		tID = fmt.Sprintf(tID + i + "_")
	}

	return fmt.Sprintf(tID + strInt)
}

// GenerateUUID generates uuid without hyphens
func GenerateUUID() string {
	id, _ := uuid.NewRandom()

	return strings.ReplaceAll(id.String(), "-", "")
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

func getUUID() string {
	id := uuid.New()
	return strings.Replace(id.String(), "-", "", -1)
}
