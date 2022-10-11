// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	tp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"

	"plugin-arch-grpc-server-go/pkg/pb"
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
	rules := &pb.Rules{Json: string(dRules)}
	codes := []string{"1", "2"}

	a := &pb.GetStatCodesRequest{Rules: rules}
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
	rules := &pb.Rules{Json: string(dRules)}
	ticket := &pb.Ticket{
		TicketId:  GenerateUUID(),
		MatchPool: "",
	}
	a := &pb.ValidateTicketRequest{
		Ticket: ticket,
		Rules:  rules,
	}
	ok, err := server.ValidateTickets(ctx, a)

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

	var players []*pb.Ticket_PlayerData
	for i := 1; i <= 4; i++ {
		p := &pb.Ticket_PlayerData{
			PlayerId:   fmt.Sprintf("player%d", i),
			Attributes: nil,
		}
		players = append(players, p)
	}

	ticket := pb.Ticket{
		TicketId:         GenerateUUID(),
		MatchPool:        "",
		CreatedAt:        &tp.Timestamp{Seconds: 10},
		Players:          players,
		TicketAttributes: nil,
		Latencies:        nil,
	}

	// act
	var tickets []pb.Ticket
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
