// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"matchmaking-function-grpc-plugin-server-go/pkg/player"

	"github.com/stretchr/testify/assert"
	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"

	"google.golang.org/grpc"
)

func TestGetStatCodes(t *testing.T) {
	// prepare
	s := grpc.NewServer()

	// act
	matchMaker := New()
	server := MatchFunctionServer{
		UnimplementedMatchFunctionServer: matchfunctiongrpc.UnimplementedMatchFunctionServer{},
		MM:                               matchMaker,
	}

	codes := []string{"2", "2"}
	ok := server.MM.GetStatCodes(codes)

	// assert
	assert.NotNil(t, s)
	assert.NotNil(t, ok)
}

func TestValidateTicket(t *testing.T) {
	// prepare
	s := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// act
	matchMaker := New()
	server := MatchFunctionServer{
		UnimplementedMatchFunctionServer: matchfunctiongrpc.UnimplementedMatchFunctionServer{},
		MM:                               matchMaker,
	}

	var rule interface{}
	dRules, _ := json.Marshal(rule)
	rules := &matchfunctiongrpc.Rules{Json: string(dRules)}
	ticket := matchmaker.Ticket{
		TicketID:  GenerateUUID(),
		MatchPool: "",
	}
	a := &matchfunctiongrpc.ValidateTicketRequest{
		Ticket: matchfunctiongrpc.MatchfunctionTicketToProtoTicket(ticket),
		Rules:  rules,
	}
	ok, err := server.ValidateTicket(ctx, a)

	// assert
	assert.NotNil(t, s)
	assert.NotNil(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, ok.ValidTicket, true)
}

func TestMatch(t *testing.T) {
	// prepare
	s := grpc.NewServer()
	server := New()
	madeMatches := 0
	ticketProvider := matchTicketProvider{make(chan matchmaker.Ticket)}
	var tickets []matchmaker.Ticket

	r := GameRules{
		ShipCountMin: 0,
		ShipCountMax: 1,
	}

	matches := server.MakeMatches(ticketProvider, r)
	var wg sync.WaitGroup
	var players []player.PlayerData

	// build tickets with only a single player
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ticketProvider.channelTickets)
		for i := 1; i <= 4; i++ {
			logrus.Infof("looping through %d time", i)
			p := player.PlayerData{
				PlayerID:   player.IDFromString(fmt.Sprintf("player%d", i)),
				PartyID:    "",
				Attributes: nil,
			}
			players = append(players, p)

			ticket := matchmaker.Ticket{
				TicketID:         GenerateUUID(),
				MatchPool:        "",
				CreatedAt:        time.Now(),
				Players:          players,
				TicketAttributes: nil,
				Latencies:        nil,
			}
			ticketProvider.channelTickets <- ticket
			tickets = append(tickets, ticket)
			players = nil
		}
	}()

	// range through matches channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		for match := range matches {
			logrus.Infof("got a match: %+v", match.Tickets)
			madeMatches++
		}
	}()

	//wait for ticket writing and matching to be done
	wg.Wait()

	// assert
	assert.NotNil(t, s)
	assert.Equal(t, len(tickets)/2, madeMatches)
}
