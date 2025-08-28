// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"time"

	"matchmaking-function-grpc-plugin-server-go/pkg/common"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	matchfunction "matchmaking-function-grpc-plugin-server-go/pkg/pb"
	"matchmaking-function-grpc-plugin-server-go/pkg/playerdata"

	pie_ "github.com/elliotchance/pie/v2"
)

// New returns a MatchMaker of the MatchLogic interface
func New() MatchLogic {
	return MatchMaker{}
}

// ValidateTicket returns a bool if the match ticket is valid
func (b MatchMaker) ValidateTicket(scope *common.Scope, matchTicket matchmaker.Ticket, matchRules interface{}) (bool, error) {
	scope.Log.Info("MATCHMAKER: validate ticket")
	scope.Log.Info("Ticket Validation successful")

	return true, nil
}

// EnrichTicket is responsible for adding logic to the match ticket before match making
func (b MatchMaker) EnrichTicket(scope *common.Scope, matchTicket matchmaker.Ticket, ruleSet interface{}) (ticket matchmaker.Ticket, err error) {
	scope.Log.Info("MATCHMAKER: enrich ticket")
	if len(matchTicket.TicketAttributes) == 0 {
		scope.Log.Info("MATCHMAKER: ticket attributes are empty, lets add some!")
		enrichMap := map[string]interface{}{
			"enrichedNumber": float64(20),
		}
		matchTicket.TicketAttributes = enrichMap
		scope.Log.Infof("EnrichedTicket Attributes: %+v", matchTicket.TicketAttributes)
	}

	return matchTicket, nil
}

// GetStatCodes returns the string slice of the stat codes in matchrules
func (b MatchMaker) GetStatCodes(scope *common.Scope, matchRules interface{}) []string {
	scope.Log.Infof("MATCHMAKER: stat codes: %s", []string{})

	return []string{}
}

// RulesFromJSON returns the ruleset from the Game rules
func (b MatchMaker) RulesFromJSON(scope *common.Scope, jsonRules string) (interface{}, error) {
	var ruleSet GameRules
	err := json.Unmarshal([]byte(jsonRules), &ruleSet)
	if err != nil {
		return nil, err
	}

	if ruleSet.AllianceRule.MinNumber > ruleSet.AllianceRule.MaxNumber {
		return nil, status.Error(codes.InvalidArgument, "alliance rule MaxNumber is less than MinNumber")
	}

	if ruleSet.AllianceRule.PlayerMinNumber > ruleSet.AllianceRule.PlayerMaxNumber {
		return nil, status.Error(codes.InvalidArgument, "alliance rule PlayerMaxNumber is less than PlayerMinNumber")
	}

	if ruleSet.ShipCountMin > ruleSet.ShipCountMax {
		return nil, status.Error(codes.InvalidArgument, "ShipCountMax is less than ShipCountMin")
	}

	return ruleSet, nil
}

// MakeMatches iterates over all the match tickets and matches them based on the buildMatch function
func (b MatchMaker) MakeMatches(scope *common.Scope, ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.Match {
	scope.Log.Info("MATCHMAKER: make matches")
	log := scope.Log.WithField("method", "MATCHMAKER.MakeMatches")
	results := make(chan matchmaker.Match)
	ctx := scope.Ctx

	rule, ok := matchRules.(GameRules)
	if !ok {
		log.Errorf("unexpected game rule type %T", matchRules)

		return nil
	}

	go func() {
		defer close(results)
		var unmatchedTickets []matchmaker.Ticket
		nextTicket := ticketProvider.GetTickets()
		for {
			select {
			case ticket, ok := <-nextTicket:
				if !ok {
					scope.Log.Info("MATCHMAKER: there are no tickets to create a match with")

					return
				}
				scope.Log.Infof("MATCHMAKER: got a ticket: %s", ticket.TicketID)
				unmatchedTickets = buildMatch(scope, ticket, unmatchedTickets, rule, results)

			case <-ctx.Done():
				scope.Log.Info("MATCHMAKER: CTX Done triggered")

				return
			}
		}
	}()

	return results
}

// buildMatch is responsible for building matches from the slice of match tickets and feeding them to the match channel
func buildMatch(scope *common.Scope, ticket matchmaker.Ticket, unmatchedTickets []matchmaker.Ticket, rule GameRules, results chan matchmaker.Match) []matchmaker.Ticket {
	scope.Log.Info("MATCHMAKER: seeing if we have enough tickets to match")
	unmatchedTickets = append(unmatchedTickets, ticket)

	minPlayers := rule.AllianceRule.MinNumber * rule.AllianceRule.PlayerMinNumber
	maxPlayers := rule.AllianceRule.MaxNumber * rule.AllianceRule.PlayerMaxNumber

	if minPlayers == 0 && maxPlayers == 0 {
		minPlayers = 2
		maxPlayers = 2
	}

	if rule.ShipCountMin == 0 {
		minPlayers *= 1
	} else {
		minPlayers *= rule.ShipCountMin
	}

	if rule.ShipCountMax == 0 {
		maxPlayers *= 1
	} else {
		maxPlayers *= rule.ShipCountMax
	}

	for {
		if len(unmatchedTickets) < minPlayers {
			break
		}

		numPlayers := minPlayers
		if len(unmatchedTickets) >= maxPlayers {
			numPlayers = maxPlayers
		}

		scope.Log.Info("MATCHMAKER: I have enough tickets to match!")

		backfill := false
		if rule.AutoBackfill && numPlayers < maxPlayers {
			backfill = true
		}

		var players []playerdata.PlayerData

		for i := 0; i < numPlayers; i++ {
			players = append(players, unmatchedTickets[i].Players...)
		}
		playerIDs := pie_.Map(players, playerdata.ToID)

		// RegionPreference value is just an example. The value(s) should be from the best region on the matchmaker.Ticket.Latencies
		teamID := common.GenerateUUID()
		match := matchmaker.Match{
			RegionPreference: []string{"us-east-2", "us-west-2"},
			Tickets:          make([]matchmaker.Ticket, numPlayers),
			Teams: []matchmaker.Team{
				{UserIDs: playerIDs, TeamID: teamID},
			},
			Backfill: backfill,
			MatchAttributes: map[string]interface{}{
				"assignment": map[string]interface{}{
					"small-team-1": []string{teamID},
				},
			},
		}
		copy(match.Tickets, unmatchedTickets)
		scope.Log.Info("MATCHMAKER: sending to results channel")
		results <- match
		scope.Log.Infof("MATCHMAKER: reducing unmatched tickets %d to %d", len(unmatchedTickets), len(unmatchedTickets)-numPlayers)
		unmatchedTickets = unmatchedTickets[numPlayers:]
	}

	scope.Log.Info("MATCHMAKER: not enough tickets to build a match")

	return unmatchedTickets
}

func (b MatchMaker) BackfillMatches(scope *common.Scope, ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.BackfillProposal {
	results := make(chan matchmaker.BackfillProposal)
	ctx := scope.Ctx

	scope.Log.WithField("method", "MatchMaker.BackfillMatches")
	scope.Log.Info("start")

	rule, ok := matchRules.(GameRules)
	if !ok {
		scope.Log.Errorf("unexpected game rule type %T", matchRules)

		return nil
	}

	go func() {
		defer func() {
			close(results)
			scope.Log.Info("end backfill")
		}()
		var unmatchedTickets []matchmaker.Ticket
		var unmatchedBackfillTickets []matchmaker.BackfillTicket
		nextTicket := ticketProvider.GetTickets()
		nextBackfillTicket := ticketProvider.GetBackfillTickets()

		for {
			if nextTicket == nil && nextBackfillTicket == nil {
				return
			}

			select {
			case ticket, ok := <-nextTicket:
				if !ok {
					scope.Log.Info("no more match tickets")
					nextTicket = nil

					continue
				}
				scope.Log.WithField("ticketId", ticket.TicketID).Infof("got a match ticket")
				unmatchedTickets, unmatchedBackfillTickets = buildBackfillMatch(scope, &ticket, nil, unmatchedTickets, unmatchedBackfillTickets, rule, results)
			case backfillTicket, ok := <-nextBackfillTicket:
				if !ok {
					scope.Log.Info("no more backfill tickets")
					nextBackfillTicket = nil

					continue
				}
				scope.Log.WithField("ticketId", backfillTicket.TicketID).Infof("got a backfill ticket")
				unmatchedTickets, unmatchedBackfillTickets = buildBackfillMatch(scope, nil, &backfillTicket, unmatchedTickets, unmatchedBackfillTickets, rule, results)
			case <-ctx.Done():
				scope.Log.Info("CTX Done triggered")

				return
			}
		}
	}()

	return results
}

// buildBackfillMatch is responsible for building matches from the slice of match tickets and feeding them to the match channel
func buildBackfillMatch(scope *common.Scope, newTicket *matchmaker.Ticket, newBackfillTicket *matchmaker.BackfillTicket, unmatchedTickets []matchmaker.Ticket, unmatchedBackfillTickets []matchmaker.BackfillTicket, rule GameRules, results chan matchmaker.BackfillProposal) ([]matchmaker.Ticket, []matchmaker.BackfillTicket) {
	log := scope.Log.WithField("method", "MATCHMAKER.buildBackfillMatch")

	if newTicket != nil {
		unmatchedTickets = append(unmatchedTickets, *newTicket)
	}
	if newBackfillTicket != nil {
		unmatchedBackfillTickets = append(unmatchedBackfillTickets, *newBackfillTicket)
	}

	log.WithField("numBackfill", len(unmatchedBackfillTickets)).
		WithField("numTicket", len(unmatchedTickets)).
		Info("buildBackfillMatch")

	if len(unmatchedBackfillTickets) > 0 && len(unmatchedTickets) > 0 {
		log.Info("I have enough tickets to backfill!")
		var i int
		var backfillTicket matchmaker.BackfillTicket
		for i, backfillTicket = range unmatchedBackfillTickets {
			ticket := unmatchedTickets[0]
			unmatchedTickets = unmatchedTickets[1:]

			proposedTeam := backfillTicket.PartialMatch.Teams
			proposedTeam = append(proposedTeam, matchmaker.Team{
				UserIDs: pie_.Map(ticket.Players, playerdata.ToID),
				Parties: matchfunction.PlayerDataToParties(ticket.Players),
				TeamID:  common.GenerateUUID(),
			})

			log.Info("Send backfill proposal!")
			results <- matchmaker.BackfillProposal{
				BackfillTicketID: backfillTicket.TicketID,
				CreatedAt:        time.Time{},
				AddedTickets:     []matchmaker.Ticket{ticket},
				ProposedTeams:    proposedTeam,
				ProposalID:       "",
				MatchPool:        backfillTicket.MatchPool,
				MatchSessionID:   backfillTicket.MatchSessionID,
			}

			if len(unmatchedTickets) == 0 {
				if i+1 < len(unmatchedBackfillTickets) {
					unmatchedBackfillTickets = unmatchedBackfillTickets[i+1:]
				} else {
					unmatchedBackfillTickets = nil
				}

				break
			}
		}
	} else {
		log.Info("not enough tickets to build a match")
	}

	return unmatchedTickets, unmatchedBackfillTickets
}
