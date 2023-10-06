// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"sort"

	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"matchmaking-function-grpc-plugin-server-go/pkg/playerdata"

	pie_ "github.com/elliotchance/pie/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
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

	return ruleSet, nil
}

// MakeMatches iterates over all the match tickets and matches them based on the buildMatch function
func (b MatchMaker) MakeMatches(ticketProvider TicketProvider, matchRules interface{}) <-chan matchmaker.Match {
	logrus.Info("MATCHMAKER: make matches")
	results := make(chan matchmaker.Match)
	ctx := context.Background()
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
				unmatchedTickets = buildMatch(ticket, unmatchedTickets, results)
			case <-ctx.Done():
				logrus.Info("MATCHMAKER: CTX Done triggered")

				return
			}
		}
	}()

	return results
}

// buildMatch is responsible for building matches from the slice of match tickets and feeding them to the match channel
func buildMatch(ticket matchmaker.Ticket, unmatchedTickets []matchmaker.Ticket, results chan matchmaker.Match) []matchmaker.Ticket {
	logrus.Info("MATCHMAKER: seeing if we have enough tickets to match")
	unmatchedTickets = append(unmatchedTickets, ticket)
	if len(unmatchedTickets) == 2 {
		logrus.Info("MATCHMAKER: I have enough tickets to match!")
		players := append(unmatchedTickets[0].Players, unmatchedTickets[1].Players...)
		playerIDs := pie_.Map(players, playerdata.ToID)
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

func (b MatchMaker) GetBestMatchRegion(tickets []matchmaker.Ticket) []string {
	logrus.Info("MATCHMAKER: get best match region")
	var regions []string
	var allRegions []string
	for _, ticket := range tickets {
		var sortedLatency []matchmaker.Region
		for region, latency := range ticket.Latencies {
			sortedLatency = append(sortedLatency, matchmaker.Region{Region: region, Latency: int(latency)})
		}
		sort.SliceStable(sortedLatency, func(i, j int) bool {
			return sortedLatency[i].Latency < sortedLatency[j].Latency
		})

		for k, latency := range sortedLatency {
			if k == 0 {
				regions = append(regions, latency.Region)
			}

			if !slices.Contains(allRegions, latency.Region) {
				allRegions = append(allRegions, latency.Region)
			}
		}
	}

	if len(regions) > 0 {
		counts := make(map[string]int)
		for _, region := range regions {
			counts[region]++
		}

		ratingRegions := make([]string, 0, len(counts))
		for rating := range counts {
			ratingRegions = append(ratingRegions, rating)
		}
		sort.Slice(ratingRegions, func(i, j int) bool { return counts[ratingRegions[i]] > counts[ratingRegions[j]] })

		diff, _ := CheckDiff(allRegions, ratingRegions)
		ratingRegions = append(ratingRegions, diff...)

		return ratingRegions
	}

	return []string{}
}
