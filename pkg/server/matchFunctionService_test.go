// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	tp "google.golang.org/protobuf/types/known/timestamppb"

	matchfunctiongrpc "plugin-arch-grpc-server-go/pkg/pb"

	"google.golang.org/grpc"
)

func TestGetStatCodes(t *testing.T) {
	// prepare
	s := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// act
	server := MatchFunctionServer{}

	var rule interface{} // needs to add rule
	dRules, _ := json.Marshal(rule)
	rules := &matchfunctiongrpc.Rules{Json: string(dRules)}
	codes := []string{"2", "2"}

	a := &matchfunctiongrpc.GetStatCodesRequest{Rules: rules}
	ok, err := server.GetStatCodes(ctx, a)

	// assert
	assert.NotNil(t, s)
	assert.NotNil(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, codes, ok.Codes)
}

func TestValidateTicket(t *testing.T) {
	// prepare
	s := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// act
	server := MatchFunctionServer{}

	var rule interface{}
	dRules, _ := json.Marshal(rule)
	rules := &matchfunctiongrpc.Rules{Json: string(dRules)}
	ticket := &matchfunctiongrpc.Ticket{
		TicketId:  GenerateUUID(),
		MatchPool: "",
	}
	a := &matchfunctiongrpc.ValidateTicketRequest{
		Ticket: ticket,
		Rules:  rules,
	}
	ok, err := server.ValidateTicket(ctx, a)

	// assert
	assert.NotNil(t, s)
	assert.NotNil(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, ok.Valid, true)
}

func TestMatch(t *testing.T) {
	// prepare
	s := grpc.NewServer()

	r := GameRules{
		ShipCountMin: 0,
		ShipCountMax: 1,
	}

	var players []*matchfunctiongrpc.Ticket_PlayerData
	for i := 1; i <= 4; i++ {
		p := &matchfunctiongrpc.Ticket_PlayerData{
			PlayerId:   fmt.Sprintf("player%d", i),
			Attributes: nil,
		}
		players = append(players, p)
	}

	ticket := matchfunctiongrpc.Ticket{
		TicketId:         GenerateUUID(),
		MatchPool:        "",
		CreatedAt:        &tp.Timestamp{Seconds: 10},
		Players:          players,
		TicketAttributes: nil,
		Latencies:        nil,
	}

	// act
	var tickets []matchfunctiongrpc.Ticket
	results := make([]Match, 0)
	tickets = append(tickets, ticket)
	server := MatchMaker{unmatchedTickets: tickets}
	matches := server.MakeMatches(r)

	for match := range matches {
		results = append(results, match)
	}

	// assert
	assert.NotNil(t, s)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, 4, len(players))
}
