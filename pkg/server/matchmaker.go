// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"plugin-arch-grpc-server-go/pkg/pb"
)

func New() MatchLogic {
	return MatchMaker{}
}

type MatchMaker struct {
	unmatchedTickets []pb.Ticket
}

type MatchLogic interface {
	MakeMatches(matchRules interface{}) <-chan Match
	RulesFromJSON(json string) (interface{}, error)
	GetStatCodes(matchRules interface{}) []string
	ValidateTicket(ticket *pb.Ticket, matchRules interface{}) (bool, error)
}

func (b MatchMaker) ValidateTicket(ticket *pb.Ticket, matchRules interface{}) (bool, error) {
	return true, nil
}

func (b MatchMaker) GetStatCodes(matchRules interface{}) []string { return []string{} }

// RulesFromJSON returns the ruleset from the Game rules
func (b MatchMaker) RulesFromJSON(jsonRules string) (interface{}, error) {
	var ruleSet GameRules

	err := json.Unmarshal([]byte(jsonRules), &ruleSet)
	if err != nil {
		return nil, err
	}
	if ruleSet.ShipCountMin == 0 {
		return nil, fmt.Errorf("ShipCountMin is 0")
	}
	if ruleSet.ShipCountMax == 0 {
		return nil, fmt.Errorf("ShipCountMax is 0")
	}

	return ruleSet, nil
}

// MakeMatches iterates over all the crew tickets and matches them based on the min/max of the game rules
func (b MatchMaker) MakeMatches(matchRules interface{}) <-chan Match {
	results := make(chan Match)
	ruleSet, ok := matchRules.(GameRules)
	if !ok {
		logrus.Error("invalid type for match rules")
		close(results)

		return results
	}

	go func() {
		defer close(results)

		unmatchedTickets := make([]*pb.Ticket, 0, int(ruleSet.ShipCountMax))
		if len(unmatchedTickets) == int(ruleSet.ShipCountMax) {
			match := buildMatch(unmatchedTickets)
			results <- match
		}
		if len(unmatchedTickets) >= int(ruleSet.ShipCountMin) {
			match := buildMatch(unmatchedTickets)
			results <- match
		}
	}()

	return results
}

type Match struct {
	Tickets           []*pb.Ticket
	Teams             []pb.Match_Team
	RegionPreferences []string
	MatchAttributes   map[string]interface{}
}

func buildMatch(unmatchedTickets []*pb.Ticket) Match {
	match := Match{
		RegionPreferences: []string{},
		Tickets:           unmatchedTickets,
		Teams:             nil,
	}

	return match
}
