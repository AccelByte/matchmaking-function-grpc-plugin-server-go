// Copyright (c) 2024 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	type args struct {
		key      string
		fallback string
	}
	tests := []struct {
		name     string
		args     args
		want     string
		envValue *string
	}{
		{
			name: "Empty value envar",
			args: args{
				key:      "BASE_URL",
				fallback: "http://localhost:9090",
			},
			envValue: pstr(""),
			want:     "",
		},
		{
			name: "Proper value envar",
			args: args{
				key:      "BASE_URL",
				fallback: "http://localhost:9090",
			},
			envValue: pstr("http://localhost"),
			want:     "http://localhost",
		},
		{
			name: "Unset envar",
			args: args{
				key:      "BASE_URL",
				fallback: "http://localhost:9090",
			},
			envValue: nil,
			want:     "http://localhost:9090",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != nil {
				t.Setenv(tt.args.key, *tt.envValue)
			}
			assert.Equalf(t, tt.want, GetEnv(tt.args.key, tt.args.fallback), "GetEnv(%v, %v)", tt.args.key, tt.args.fallback)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	type args struct {
		key      string
		fallback int
	}
	tests := []struct {
		name     string
		args     args
		envValue *string
		want     int
	}{
		{
			name: "Empty value envar",
			args: args{
				key:      "MAX_COUNT",
				fallback: 12,
			},
			envValue: pstr(""),
			want:     12,
		},
		{
			name: "Proper value envar",
			args: args{
				key:      "MAX_COUNT",
				fallback: 150,
			},
			envValue: pstr("51"),
			want:     51,
		},
		{
			name: "Non integer value envar",
			args: args{
				key:      "MAX_COUNT",
				fallback: 123,
			},
			envValue: pstr("non-integer"),
			want:     123,
		},
		{
			name: "Unset envar",
			args: args{
				key:      "MAX_COUNT",
				fallback: 100,
			},
			envValue: nil,
			want:     100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != nil {
				t.Setenv(tt.args.key, *tt.envValue)
			}
			assert.Equalf(t, tt.want, GetEnvInt(tt.args.key, tt.args.fallback), "GetEnvInt(%v, %v)", tt.args.key, tt.args.fallback)
		})
	}
}

func pstr(s string) *string {
	return &s
}
