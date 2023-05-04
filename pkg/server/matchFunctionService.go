// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"io"
	"sync"

	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"

	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"

	"github.com/sirupsen/logrus"
)

// MatchFunctionServer is for the handler (upper level of match logic)
type MatchFunctionServer struct {
	matchfunctiongrpc.UnimplementedMatchFunctionServer
	MM MatchLogic

	shipCountMin     int
	shipCountMax     int
	unmatchedTickets []*matchmaker.Ticket
}

// matchTicketProvider contains the go channel of matchmaker tickets needed for making matches
type matchTicketProvider struct {
	channelTickets chan matchmaker.Ticket
}

// GetTickets will return the go channel of tickets from the matchTicketProvider
func (m matchTicketProvider) GetTickets() chan matchmaker.Ticket {
	return m.channelTickets
}

// GetBackfillTickets
func (m matchTicketProvider) GetBackfillTickets() chan matchmaker.BackfillTicket {
	c := make(chan matchmaker.BackfillTicket)
	close(c)

	return c
}

// GetStatCodes uses the assigned MatchMaker to get the stat codes of the ruleset
func (m *MatchFunctionServer) GetStatCodes(ctx context.Context, req *matchfunctiongrpc.GetStatCodesRequest) (*matchfunctiongrpc.StatCodesResponse, error) {
	rules, err := m.MM.RulesFromJSON(req.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)

		return nil, err
	}

	codes := m.MM.GetStatCodes(rules)

	return &matchfunctiongrpc.StatCodesResponse{Codes: codes}, nil
}

// ValidateTicket uses the assigned MatchMaker to validate the ticket
func (m *MatchFunctionServer) ValidateTicket(ctx context.Context, req *matchfunctiongrpc.ValidateTicketRequest) (*matchfunctiongrpc.ValidateTicketResponse, error) {
	logrus.Info("GRPC SERVICE: validate ticket")

	rules, err := m.MM.RulesFromJSON(req.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)
	}

	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)

	validTicket, err := m.MM.ValidateTicket(matchTicket, rules)

	return &matchfunctiongrpc.ValidateTicketResponse{ValidTicket: validTicket}, err
}

// EnrichTicket uses the assigned MatchMaker to enrich the ticket
func (m *MatchFunctionServer) EnrichTicket(ctx context.Context, req *matchfunctiongrpc.EnrichTicketRequest) (*matchfunctiongrpc.EnrichTicketResponse, error) {
	logrus.Info("GRPC SERVICE: enrich ticket")
	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)
	enrichedTicket, err := m.MM.EnrichTicket(matchTicket, req.Rules)
	if err != nil {
		return nil, err
	}
	newTicket := matchfunctiongrpc.MatchfunctionTicketToProtoTicket(enrichedTicket)

	return &matchfunctiongrpc.EnrichTicketResponse{Ticket: newTicket}, nil
}

// MakeMatches uses the assigned MatchMaker to build matches and sends them back to the client
func (m *MatchFunctionServer) MakeMatches(server matchfunctiongrpc.MatchFunction_MakeMatchesServer) error {
	logrus.Info("GRPC SERVICE: make matches")
	matchesMade := 0

	in, err := server.Recv()
	if err != nil {
		logrus.Errorf("error during stream Recv: %s", err)

		return err
	}

	mrpT, ok := in.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Parameters)
	if !ok {
		logrus.Error("not a MakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	// scope := envelope.NewRootScope(context.Background(), "GRPC.MakeMatches", mrpT.Parameters.Scope.AbTraceId)
	//defer scope.Finish()

	rules, err := m.MM.RulesFromJSON(mrpT.Parameters.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)

		return err
	}

	ticketProvider := matchTicketProvider{make(chan matchmaker.Ticket)}
	resultChan := m.MM.MakeMatches(ticketProvider, rules)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			req, err := server.Recv()
			if err == io.EOF {
				logrus.Infof("GRPC SERVICE: %s", err)
				close(ticketProvider.channelTickets)

				return
			}
			if err != nil {
				logrus.Errorf("GRPC SERVICE: recv %s", err)

				return
			}
			t, ok := req.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Ticket)
			if !ok {
				logrus.Errorf("not a MakeMatchesRequest_Ticket: %s", t.Ticket)

				return
			}

			logrus.Info("GRPC SERVICE: crafting a matchfunctions.Ticket")
			matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(t.Ticket)
			logrus.Infof("GRPC SERVICE: writing match ticket: %+v", matchTicket)
			ticketProvider.channelTickets <- matchTicket
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChan {
			logrus.Info("GRPC SERVICE: crafting a MatchResponse")
			resp := matchfunctiongrpc.MatchResponse{Match: matchfunctiongrpc.MatchfunctionMatchToProtoMatch(result)}
			logrus.Infof("GRPC SERVICE: match made and being sent back to the client: %+v", &resp)
			if err := server.Send(&resp); err != nil {
				logrus.Errorf("error on server send: %s", err)

				return
			}
			matchesMade++
		}
	}()
	wg.Wait()

	logrus.Infof("GRPC SERVICE: make matches finished and %d matches were made", matchesMade)

	return nil
}

// BackfillMatches uses the assigned MatchMaker to run backfill
func (m *MatchFunctionServer) BackfillMatches(server matchfunctiongrpc.MatchFunction_BackfillMatchesServer) error {
	ctx := server.Context()
	defer ctx.Done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		in, err := server.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if backfillTicket := in.GetBackfillTicket(); backfillTicket != nil {
			proposal := &matchfunctiongrpc.BackfillResponse{
				BackfillProposal: &matchfunctiongrpc.BackfillProposal{},
			}
			if err := server.Send(proposal); err != nil {
				return err
			}
		}
	}
}
