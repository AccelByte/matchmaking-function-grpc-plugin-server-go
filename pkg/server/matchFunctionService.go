// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"errors"
	"io"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"sync"

	"github.com/sirupsen/logrus"
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

type matchTicketProvider struct {
	channelTickets chan matchmaker.Ticket
}

func (m matchTicketProvider) GetTickets() chan matchmaker.Ticket {
	return m.channelTickets
}

func (m matchTicketProvider) GetBackfillTickets() chan matchmaker.BackfillTicket {
	c := make(chan matchmaker.BackfillTicket)
	close(c)
	return c
}

func (m *MatchFunctionServer) GetStatCodes(ctx context.Context, req *matchfunctiongrpc.GetStatCodesRequest) (*matchfunctiongrpc.StatCodesResponse, error) {

	rules, err := m.MM.RulesFromJSON(req.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)
		return nil, err
	}

	codes := m.MM.GetStatCodes(rules)
	logrus.Infof("stat codes: %s", codes)
	return &matchfunctiongrpc.StatCodesResponse{Codes: codes}, nil
}

func (m *MatchFunctionServer) ValidateTicket(ctx context.Context, req *matchfunctiongrpc.ValidateTicketRequest) (*matchfunctiongrpc.ValidateTicketResponse, error) {
	logrus.Info("SERVER: validate ticket")
	//return &matchfunctiongrpc.ValidateTicketResponse{ValidTicket: true}, nil

	rules, err := m.MM.RulesFromJSON(req.Rules.Json)
	if err != nil {
		logrus.Errorf("could not get rules from json: %s", err)
	}

	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)

	logrus.Infof("ValidateTicket in Namespace: %s", matchTicket.Namespace)

	validTicket, err := m.MM.ValidateTicket(matchTicket, rules)
	return &matchfunctiongrpc.ValidateTicketResponse{ValidTicket: validTicket}, err
}

func (m *MatchFunctionServer) EnrichTicket(ctx context.Context, req *matchfunctiongrpc.EnrichTicketRequest) (*matchfunctiongrpc.EnrichTicketResponse, error) {
	logrus.Info("SERVER: enrich ticket")
	//// this will enrich ticket with these hardcoded ticket attributes
	//enrichMap := map[string]*structpb.Value{
	//	"mmr":        structpb.NewNumberValue(250.0),
	//	"teamrating": structpb.NewNumberValue(2000.0),
	//}
	//
	//if req.Ticket.TicketAttributes == nil || req.Ticket.TicketAttributes.Fields == nil {
	//	req.Ticket.TicketAttributes = &structpb.Struct{Fields: enrichMap}
	//} else {
	//	for key, value := range enrichMap {
	//		req.Ticket.TicketAttributes.Fields[key] = value
	//	}
	//}
	//return &matchfunctiongrpc.EnrichTicketResponse{Ticket: req.Ticket}, nil
	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)
	enrichedTicket, err := m.MM.EnrichTicket(matchTicket, req.Rules)
	if err != nil {
		return nil, err
	}
	newTicket := matchfunctiongrpc.MatchfunctionTicketToProtoTicket(enrichedTicket)
	return &matchfunctiongrpc.EnrichTicketResponse{Ticket: newTicket}, nil
}

func (m *MatchFunctionServer) MakeMatches(server matchfunctiongrpc.MatchFunction_MakeMatchesServer) error {
	logrus.Info("SERVER: make matches")
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

	//scope := envelope.NewRootScope(context.Background(), "GRPC.MakeMatches", mrpT.Parameters.Scope.AbTraceId)
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
				logrus.Infof("SERVER: %s", err)
				close(ticketProvider.channelTickets)
				return
			}
			if err != nil {
				logrus.Errorf("SERVER: recv %s", err)
				return
			}
			t, ok := req.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Ticket)
			if !ok {
				logrus.Errorf("not a MakeMatchesRequest_Ticket: %s", t.Ticket)
				return
			}

			logrus.Info("SERVER: crafting a matchfunctions.Ticket")
			matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(t.Ticket)
			logrus.Infof("SERVER: writing match ticket: %+v", matchTicket)
			ticketProvider.channelTickets <- matchTicket
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChan {
			logrus.Info("SERVER: crafting a MatchResponse")
			resp := matchfunctiongrpc.MatchResponse{Match: matchfunctiongrpc.MatchfunctionMatchToProtoMatch(result)}
			logrus.Infof("SERVER: match made and being sent back to the client: %+v", &resp)
			if err := server.Send(&resp); err != nil {
				logrus.Errorf("error on server send: %s", err)
				return
			}
			matchesMade++
		}
	}()
	wg.Wait()

	logrus.Infof("SERVER: make matches finished and %d matches were made", matchesMade)
	return nil

}

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
