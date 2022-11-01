// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

type GameRules struct {
	ShipCountMin float64
	ShipCountMax float64
}

type RuleObject interface {
	GetShipCountMax() (shipCountMix int)
	GetShipCountMin() (shipCountMin int)
	SetShipCountMax(shipCountMax int)
	SetShipCountMin(shipCountMin int)
}
