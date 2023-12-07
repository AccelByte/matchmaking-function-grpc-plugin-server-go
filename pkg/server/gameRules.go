// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

type AllianceRule struct {
	MinNumber       int `json:"min_number" valid:"range(0|2147483647)"`
	MaxNumber       int `json:"max_number" valid:"range(0|2147483647)"`
	PlayerMinNumber int `json:"player_min_number" valid:"range(0|2147483647)"`
	PlayerMaxNumber int `json:"player_max_number" valid:"range(0|2147483647)"`
}

type GameRules struct {
	ShipCountMin int          `json:"shipCountMin"`
	ShipCountMax int          `json:"shipCountMax"`
	AutoBackfill bool         `json:"auto_backfill"`
	AllianceRule AllianceRule `json:"alliance"`
}
