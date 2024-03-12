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

	"matchmaking-function-grpc-plugin-server-go/pkg/common"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"
	"matchmaking-function-grpc-plugin-server-go/pkg/playerdata"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"
)

func TestGetStatCodes(t *testing.T) {
	// prepare
	s := grpc.NewServer()

	scope := common.NewRootScope(context.Background(), "test", common.GenerateUUID())
	defer scope.Finish()

	// act
	matchMaker := New()
	server := MatchFunctionServer{
		UnimplementedMatchFunctionServer: matchfunctiongrpc.UnimplementedMatchFunctionServer{},
		MM:                               matchMaker,
	}

	codes := []string{"2", "2"}
	ok := server.MM.GetStatCodes(scope, codes)

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
		TicketID:  common.GenerateUUID(),
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
	scope := common.NewRootScope(context.Background(), "test", common.GenerateUUID())
	defer scope.Finish()

	s := grpc.NewServer()
	server := New()
	madeMatches := 0
	ticketProvider := newMatchTicketProvider()
	var tickets []matchmaker.Ticket

	r := GameRules{
		ShipCountMin: 0,
		ShipCountMax: 1,
	}

	matches := server.MakeMatches(scope, ticketProvider, r)
	var wg sync.WaitGroup
	var players []playerdata.PlayerData

	// build tickets with only a single playerdata
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ticketProvider.channelTickets)
		for i := 1; i <= 4; i++ {
			logrus.Infof("looping through %d time", i)
			p := playerdata.PlayerData{
				PlayerID:   playerdata.IDFromString(fmt.Sprintf("playerdata%d", i)),
				PartyID:    "",
				Attributes: nil,
			}
			players = append(players, p)

			ticket := matchmaker.Ticket{
				TicketID:         common.GenerateUUID(),
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

	// wait for ticket writing and matching to be done
	wg.Wait()

	// assert
	assert.NotNil(t, s)
	assert.Equal(t, len(tickets)/2, madeMatches)
}
