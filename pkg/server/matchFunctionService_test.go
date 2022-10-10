// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"testing"

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
	rules := &pb.Rules{Json: "foo"}
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
	rules := &pb.Rules{Json: "foo"}
	ticket := &pb.Ticket{
		TicketId:  "foo",
		MatchPool: "bar",
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

	// act
	server := MatchFunctionServer{}
	var stream grpc.ServerStream
	err := server.MakeMatches(&matchFunctionMakeMatchesServer{stream})

	// assert
	assert.NotNil(t, s)
	assert.Nil(t, err)
}
