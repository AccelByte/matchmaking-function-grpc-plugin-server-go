// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"github.com/elliotchance/pie/v2"
	"github.com/sirupsen/logrus"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"matchmaking-function-grpc-plugin-server-go/pkg/player"
)

func New() MatchLogic {
	return MatchMaker{}
}

func (b MatchMaker) ValidateTicket(matchTicket matchmaker.Ticket, matchRules interface{}) (bool, error) {
	logrus.Info("MATCHMAKER: validate ticket")
	logrus.Info("Ticket Validation successful")
	return true, nil
}

func (b MatchMaker) EnrichTicket(matchTicket matchmaker.Ticket, ruleSet interface{}) (ticket matchmaker.Ticket, err error) {
	logrus.Info("MATCHMAKER: enrich ticket")
	if len(matchTicket.TicketAttributes) == 0 {
		logrus.Info("MATCHMAKER: ticket attributes are empty, lets add some!")
		enrichMap := map[string]interface{}{
			"enrichedNumber": float64(20),
		}
		matchTicket.TicketAttributes = enrichMap
		logrus.Infof("EnrichedTicket Attributes: %+v", matchTicket.TicketAttributes)
	}
	return matchTicket, nil
}

func (b MatchMaker) GetStatCodes(matchRules interface{}) []string {
	return []string{}
}

// RulesFromJSON returns the ruleset from the Game rules
func (b MatchMaker) RulesFromJSON(jsonRules string) (interface{}, error) {
	var ruleSet GameRules
	err := json.Unmarshal([]byte(jsonRules), &ruleSet)
	if err != nil {
		return nil, err
	}
	//if ruleSet.ShipCountMin == 0 {
	//	return nil, fmt.Errorf("ShipCountMin is 0")
	//}
	//if ruleSet.ShipCountMax == 0 {
	//	return nil, fmt.Errorf("ShipCountMax is 0")
	//}

	return ruleSet, nil
}

// MakeMatches iterates over all the crew tickets and matches them based on the min/max of the game rules
func (b MatchMaker) MakeMatches(ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.Match {
	logrus.Info("MATCHMAKER: make matches")
	results := make(chan matchmaker.Match)
	ctx := context.Background()
	//TODO stream this like we do in Bleve Matchmaker
	go func() {
		defer close(results)
		var unmatchedTickets []matchmaker.Ticket
		nextTicket := ticketProvider.GetTickets()
		for {
			select {
			case ticket, ok := <-nextTicket:
				if !ok {
					logrus.Info("MATCHMAKER: there are no tickets to create a match with")
					return
				}
				logrus.Infof("MATCHMAKER: got a ticket: %s", ticket.TicketID)
				unmatchedTickets = matchTicket(ticket, unmatchedTickets, results)
			case <-ctx.Done():
				logrus.Info("MATCHMAKER: CTX Done triggered")
				return
			}
		}
	}()
	return results
	//_, cancel := context.WithCancel(context.Background())
	//defer cancel()
	//
	//results := make(chan matchmaker.Match)
	//ruleSet, ok := matchRules.(GameRules)
	//if !ok {
	//	logrus.Error("invalid type for match rules")
	//	close(results)
	//
	//	return results
	//}
	//
	//go func() {
	//	defer close(results)
	//
	//	unmatchedTickets := make([]matchmaker.Ticket, 0, int(ruleSet.ShipCountMax))
	//	if len(unmatchedTickets) == int(ruleSet.ShipCountMax) {
	//		match := buildMatch(unmatchedTickets)
	//		results <- match
	//	}
	//	if len(unmatchedTickets) >= int(ruleSet.ShipCountMin) {
	//		match := buildMatch(unmatchedTickets)
	//		results <- match
	//	}
	//}()
	//
	//return results
}

//type Match struct {
//	Tickets           []*matchfunctiongrpc.Ticket
//	Teams             []matchfunctiongrpc.Match_Team
//	RegionPreferences []string
//	MatchAttributes   map[string]interface{}
//}

func buildMatch(unmatchedTickets []matchmaker.Ticket) matchmaker.Match {
	match := matchmaker.Match{
		Tickets: unmatchedTickets,
		Teams:   nil,
	}

	return match
}

func matchTicket(ticket matchmaker.Ticket, unmatchedTickets []matchmaker.Ticket, results chan matchmaker.Match) []matchmaker.Ticket {
	logrus.Info("MATCHMAKER: seeing if we have enough tickets to match")
	unmatchedTickets = append(unmatchedTickets, ticket)
	if len(unmatchedTickets) == 2 {
		logrus.Info("MATCHMAKER: I have enough tickets to match!")
		players := append(unmatchedTickets[0].Players, unmatchedTickets[1].Players...)
		playerIDs := pie.Map(players, player.ToID)
		match := matchmaker.Match{
			RegionPreference: []string{"any"},
			Tickets:          make([]matchmaker.Ticket, 2),
			Teams: []matchmaker.Team{
				{UserIDs: playerIDs},
			},
		}
		copy(match.Tickets, unmatchedTickets)
		logrus.Info("MATCHMAKER: sending to results channel")
		results <- match
		logrus.Info("MATCHMAKER: resetting unmatched tickets")
		unmatchedTickets = nil
	}
	logrus.Info("MATCHMAKER: not enough tickets to build a match")
	return unmatchedTickets
}
