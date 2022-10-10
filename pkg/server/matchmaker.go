// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import "plugin-arch-grpc-server-go/pkg/pb"

type MatchLogic interface {
	MakeMatches(matchRules interface{}) <-chan pb.Match
	RulesFromJson(json string) (interface{}, error)
	GetStatCodes(matchRules interface{}) []string
	ValidateTicket(ticket pb.Ticket, matchRules interface{}) (bool, error)
}

type RuleObject interface {
	GetShipCountMax() (shipCountMix int)
	GetShipCountMin() (shipCountMin int)
	SetShipCountMax(shipCountMax int)
	SetShipCountMin(shipCountMin int)
}
