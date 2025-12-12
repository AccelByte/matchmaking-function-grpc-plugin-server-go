// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"matchmaking-function-grpc-plugin-server-go/pkg/common"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"
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
	channelTickets         chan matchmaker.Ticket
	channelBackfillTickets chan matchmaker.BackfillTicket
}

func newMatchTicketProvider() matchTicketProvider {
	return matchTicketProvider{
		channelTickets:         make(chan matchmaker.Ticket),
		channelBackfillTickets: make(chan matchmaker.BackfillTicket),
	}
}

// GetTickets will return the go channel of tickets from the matchTicketProvider
func (m matchTicketProvider) GetTickets() chan matchmaker.Ticket {
	return m.channelTickets
}

// GetBackfillTickets
func (m matchTicketProvider) GetBackfillTickets() chan matchmaker.BackfillTicket {
	return m.channelBackfillTickets
}

// GetStatCodes uses the assigned MatchMaker to get the stat codes of the ruleset
func (m *MatchFunctionServer) GetStatCodes(ctx context.Context, req *matchfunctiongrpc.GetStatCodesRequest) (*matchfunctiongrpc.StatCodesResponse, error) {
	scope := common.ChildScopeFromRemoteScope(ctx, "MatchFunctionServer.GetStatCodes")
	defer scope.Finish()

	rules, err := m.MM.RulesFromJSON(scope, req.Rules.Json)
	if err != nil {
		scope.Log.Error("could not get rules from json", "error", err)

		return nil, err
	}

	codes := m.MM.GetStatCodes(scope, rules)

	return &matchfunctiongrpc.StatCodesResponse{Codes: codes}, nil
}

// ValidateTicket uses the assigned MatchMaker to validate the ticket
func (m *MatchFunctionServer) ValidateTicket(ctx context.Context, req *matchfunctiongrpc.ValidateTicketRequest) (*matchfunctiongrpc.ValidateTicketResponse, error) {
	scope := common.ChildScopeFromRemoteScope(ctx, "MatchFunctionServer.ValidateTicket")
	defer scope.Finish()

	scope.Log.Info("GRPC SERVICE: validate ticket")

	rules, err := m.MM.RulesFromJSON(scope, req.Rules.Json)
	if err != nil {
		scope.Log.Error("could not get rules from json", "error", err)
	}

	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)

	validTicket, err := m.MM.ValidateTicket(scope, matchTicket, rules)

	return &matchfunctiongrpc.ValidateTicketResponse{ValidTicket: validTicket}, err
}

// EnrichTicket uses the assigned MatchMaker to enrich the ticket
func (m *MatchFunctionServer) EnrichTicket(ctx context.Context, req *matchfunctiongrpc.EnrichTicketRequest) (*matchfunctiongrpc.EnrichTicketResponse, error) {
	scope := common.ChildScopeFromRemoteScope(ctx, "MatchFunctionServer.EnrichTicket")
	defer scope.Finish()

	scope.Log.Info("GRPC SERVICE: enrich ticket", "ticket", req.Ticket)
	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)
	enrichedTicket, err := m.MM.EnrichTicket(scope, matchTicket, req.Rules)
	if err != nil {
		return nil, err
	}
	newTicket := matchfunctiongrpc.MatchfunctionTicketToProtoTicket(enrichedTicket)

	response := &matchfunctiongrpc.EnrichTicketResponse{Ticket: newTicket}
	scope.Log.Info("Response enrich ticket", "response", response)

	return response, nil
}

// MakeMatches uses the assigned MatchMaker to build matches and sends them back to the client
func (m *MatchFunctionServer) MakeMatches(server matchfunctiongrpc.MatchFunction_MakeMatchesServer) error {
	scope := common.ChildScopeFromRemoteScope(context.Background(), "MatchFunctionServer.MakeMatches")
	defer scope.Finish()

	matchesMade := 0

	in, err := server.Recv()
	if err != nil {
		scope.Log.Error("error during stream Recv", "error", err)

		return err
	}

	mrpT, ok := in.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Parameters)
	if !ok {
		scope.Log.Error("not a MakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	// scope := envelope.NewRootScope(context.Background(), "GRPC.MakeMatches", mrpT.Parameters.Scope.AbTraceId)
	//defer scope.Finish()

	rules, err := m.MM.RulesFromJSON(scope, mrpT.Parameters.Rules.Json)
	if err != nil {
		scope.Log.Error("could not get rules from json", "error", err)

		return err
	}

	scope.Log.Info("Retrieved rules", "rules", rules)

	ticketProvider := newMatchTicketProvider()
	resultChan := m.MM.MakeMatches(scope, ticketProvider, rules)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			close(ticketProvider.channelTickets)
			close(ticketProvider.channelBackfillTickets)
		}()

		for {
			req, err := server.Recv()
			if err == io.EOF {
				scope.Log.Debug("Recv ended")

				return
			}
			if err != nil {
				scope.Log.Debug("Recv error", "error", err)

				return
			}
			t, ok := req.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Ticket)
			if !ok {
				scope.Log.Error("not a MakeMatchesRequest_Ticket", "ticket", t.Ticket)

				return
			}

			scope.Log.Info("crafting a matchfunctions.Ticket")
			matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(t.Ticket)
			scope.Log.Info("writing match ticket", "matchTicket", matchTicket)
			ticketProvider.channelTickets <- matchTicket
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChan {
			scope.Log.Info("crafting a MatchResponse")
			resp := matchfunctiongrpc.MatchResponse{Match: matchfunctiongrpc.MatchfunctionMatchToProtoMatch(result)}
			scope.Log.Info("Response", "response", resp)
			scope.Log.Info("match made and being sent back to the client", "response", &resp)
			if err := server.Send(&resp); err != nil {
				scope.Log.Error("error on server send", "error", err)

				return
			}
			matchesMade++
		}
	}()
	wg.Wait()

	scope.Log.Info("make matches finished", "matchesMade", matchesMade)

	return nil
}

// BackfillMatches uses the assigned MatchMaker to run backfill
func (m *MatchFunctionServer) BackfillMatches(server matchfunctiongrpc.MatchFunction_BackfillMatchesServer) error {
	scope := common.ChildScopeFromRemoteScope(context.Background(), "MatchFunctionServer.BackfillMatches")
	defer scope.Finish()

	scope.Log.Info("backfill matches")

	in, err := server.Recv()
	if err == io.EOF {
		scope.Log.Debug("Recv ended")

		return nil
	}
	if err != nil {
		scope.Log.Error("Recv error", "error", err)

		return err
	}

	mrpT, ok := in.GetRequestType().(*matchfunctiongrpc.BackfillMakeMatchesRequest_Parameters)
	if !ok {
		scope.Log.Error("not a BackfillMakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	rules, err := m.MM.RulesFromJSON(scope, mrpT.Parameters.Rules.Json)
	if err != nil {
		scope.Log.Error("could not get rules from json", "error", err)

		return err
	}

	scope.Log.Info("Retrieved rules", "rules", rules)

	ticketProvider := newMatchTicketProvider()

	go m.fetchBackfillTickets(scope.Ctx, ticketProvider, server)

	backfillProposal := m.MM.BackfillMatches(scope, ticketProvider, rules)
	for {
		proposal, ok := <-backfillProposal
		if !ok {
			scope.Log.Info("no more proposal")

			return nil
		}

		resp := matchfunctiongrpc.BackfillResponse{
			BackfillProposal: matchfunctiongrpc.MatchfunctionBackfillProposalToProtoBackfillProposal(proposal),
		}

		scope.Log.Info("send proposal", "proposal", proposal)

		err = server.Send(&resp)
		if err != nil {
			scope.Log.Error("send proposal error", "error", err)

			return err
		}
	}
}

func (m *MatchFunctionServer) fetchBackfillTickets(ctx context.Context, ticketProvider matchTicketProvider, server matchfunctiongrpc.MatchFunction_BackfillMatchesServer) {
	log := slog.Default()

	defer func() {
		close(ticketProvider.channelTickets)
		close(ticketProvider.channelBackfillTickets)
	}()

	for {
		in, err := server.Recv()
		if err == io.EOF {
			log.Info("Ticket Recv ended")

			return
		}
		if err != nil {
			log.Error("Recv error", "error", err)

			return
		}

		if ticket := in.GetTicket(); ticket != nil {
			t := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(ticket)
			log.Info("Received match ticket",
				"matchpool", t.MatchPool,
				"ticketId", t.TicketID)
			ticketProvider.channelTickets <- t
		} else if backfillTicket := in.GetBackfillTicket(); backfillTicket != nil {
			t := matchfunctiongrpc.ProtoBackfillTicketToMatchfunctionBackfillTicket(backfillTicket)
			log.Info("Received backfill ticket",
				"matchpool", t.MatchPool,
				"ticketId", t.TicketID)
			ticketProvider.channelBackfillTickets <- t
		}
	}
}
