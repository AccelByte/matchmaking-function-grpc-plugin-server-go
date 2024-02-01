// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"

	pie_ "github.com/elliotchance/pie/v2"
	"github.com/sirupsen/logrus"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	matchfunction "matchmaking-function-grpc-plugin-server-go/pkg/pb"
	"matchmaking-function-grpc-plugin-server-go/pkg/playerdata"
)

// New returns a MatchMaker of the MatchLogic interface
func New() MatchLogic {
	return MatchMaker{}
}

// ValidateTicket returns a bool if the match ticket is valid
func (b MatchMaker) ValidateTicket(matchTicket matchmaker.Ticket, matchRules interface{}) (bool, error) {
	logrus.Info("MATCHMAKER: validate ticket")
	logrus.Info("Ticket Validation successful")

	return true, nil
}

// EnrichTicket is responsible for adding logic to the match ticket before match making
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

// GetStatCodes returns the string slice of the stat codes in matchrules
func (b MatchMaker) GetStatCodes(matchRules interface{}) []string {
	logrus.Infof("MATCHMAKER: stat codes: %s", []string{})

	return []string{}
}

// RulesFromJSON returns the ruleset from the Game rules
func (b MatchMaker) RulesFromJSON(jsonRules string) (interface{}, error) {
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
func (b MatchMaker) MakeMatches(ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.Match {
	logrus.Info("MATCHMAKER: make matches")
	log := logrus.WithField("method", "MATCHMAKER.MakeMatches")
	results := make(chan matchmaker.Match)
	ctx := context.Background()

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
					logrus.Info("MATCHMAKER: there are no tickets to create a match with")

					return
				}
				logrus.Infof("MATCHMAKER: got a ticket: %s", ticket.TicketID)
				unmatchedTickets = buildMatch(ticket, unmatchedTickets, rule, results)

			case <-ctx.Done():
				logrus.Info("MATCHMAKER: CTX Done triggered")

				return
			}
		}
	}()

	return results
}

// buildMatch is responsible for building matches from the slice of match tickets and feeding them to the match channel
func buildMatch(ticket matchmaker.Ticket, unmatchedTickets []matchmaker.Ticket, rule GameRules, results chan matchmaker.Match) []matchmaker.Ticket {
	logrus.Info("MATCHMAKER: seeing if we have enough tickets to match")
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

		logrus.Info("MATCHMAKER: I have enough tickets to match!")

		backfill := false
		if rule.AutoBackfill && numPlayers < maxPlayers {
			backfill = true
		}

		var players []playerdata.PlayerData

		for i := 0; i < numPlayers; i++ {
			players = append(players, unmatchedTickets[i].Players...)
		}
		playerIDs := pie_.Map(players, playerdata.ToID)
		match := matchmaker.Match{
			RegionPreference: []string{"any"},
			Tickets:          make([]matchmaker.Ticket, numPlayers),
			Teams: []matchmaker.Team{
				{UserIDs: playerIDs},
			},
			Backfill: backfill,
		}
		copy(match.Tickets, unmatchedTickets)
		logrus.Info("MATCHMAKER: sending to results channel")
		results <- match
		logrus.Infof("MATCHMAKER: reducing unmatched tickets %d to %d", len(unmatchedTickets), len(unmatchedTickets)-numPlayers)
		unmatchedTickets = unmatchedTickets[numPlayers:]
	}

	logrus.Info("MATCHMAKER: not enough tickets to build a match")

	return unmatchedTickets
}

func (b MatchMaker) BackfillMatches(ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.BackfillProposal {
	results := make(chan matchmaker.BackfillProposal)
	ctx := context.Background()

	log := logrus.WithField("method", "MatchMaker.BackfillMatches")
	log.Info("start")

	rule, ok := matchRules.(GameRules)
	if !ok {
		log.Errorf("unexpected game rule type %T", matchRules)
		return nil
	}

	go func() {
		defer func() {
			close(results)
			log.Info("end backfill")
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
					log.Info("no more match tickets")
					nextTicket = nil
					continue
				}
				log.WithField("ticketId", ticket.TicketID).Infof("got a match ticket")
				unmatchedTickets, unmatchedBackfillTickets = buildBackfillMatch(&ticket, nil, unmatchedTickets, unmatchedBackfillTickets, rule, results)
			case backfillTicket, ok := <-nextBackfillTicket:
				if !ok {
					log.Info("no more backfill tickets")
					nextBackfillTicket = nil
					continue
				}
				log.WithField("ticketId", backfillTicket.TicketID).Infof("got a backfill ticket")
				unmatchedTickets, unmatchedBackfillTickets = buildBackfillMatch(nil, &backfillTicket, unmatchedTickets, unmatchedBackfillTickets, rule, results)
			case <-ctx.Done():
				log.Info("CTX Done triggered")

				return
			}
		}
	}()

	return results
}

// buildBackfillMatch is responsible for building matches from the slice of match tickets and feeding them to the match channel
func buildBackfillMatch(newTicket *matchmaker.Ticket, newBackfillTicket *matchmaker.BackfillTicket, unmatchedTickets []matchmaker.Ticket, unmatchedBackfillTickets []matchmaker.BackfillTicket, rule GameRules, results chan matchmaker.BackfillProposal) ([]matchmaker.Ticket, []matchmaker.BackfillTicket) {
	log := logrus.WithField("method", "MATCHMAKER.buildBackfillMatch")

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
