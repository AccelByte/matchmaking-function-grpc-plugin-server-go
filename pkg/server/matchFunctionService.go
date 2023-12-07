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
	logrus.Infof("GRPC SERVICE: enrich ticket: %s \n", logJSONFormatter(req.Ticket))
	matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(req.Ticket)
	enrichedTicket, err := m.MM.EnrichTicket(matchTicket, req.Rules)
	if err != nil {
		return nil, err
	}
	newTicket := matchfunctiongrpc.MatchfunctionTicketToProtoTicket(enrichedTicket)

	response := &matchfunctiongrpc.EnrichTicketResponse{Ticket: newTicket}
	logrus.Infof("Response enrich ticket: %s \n", logJSONFormatter(response))

	return response, nil
}

// MakeMatches uses the assigned MatchMaker to build matches and sends them back to the client
func (m *MatchFunctionServer) MakeMatches(server matchfunctiongrpc.MatchFunction_MakeMatchesServer) error {
	log := logrus.WithField("method", "GRPC_SERVICE.MakeMatches")
	log.Info("make matches")
	matchesMade := 0

	in, err := server.Recv()
	if err != nil {
		log.WithError(err).Error("error during stream Recv")

		return err
	}

	mrpT, ok := in.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Parameters)
	if !ok {
		log.Error("not a MakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	// scope := envelope.NewRootScope(context.Background(), "GRPC.MakeMatches", mrpT.Parameters.Scope.AbTraceId)
	//defer scope.Finish()

	rules, err := m.MM.RulesFromJSON(mrpT.Parameters.Rules.Json)
	if err != nil {
		log.WithError(err).Error("could not get rules from json")

		return err
	}

	log.WithField("rules", logJSONFormatter(rules)).Infof("Retrieved rules")

	ticketProvider := newMatchTicketProvider()
	resultChan := m.MM.MakeMatches(ticketProvider, rules)
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
				log.Debug("Recv ended")

				return
			}
			if err != nil {
				log.WithError(err).Debug("Recv error")

				return
			}
			t, ok := req.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Ticket)
			if !ok {
				log.Errorf("not a MakeMatchesRequest_Ticket: %s", t.Ticket)

				return
			}

			log.Info("crafting a matchfunctions.Ticket")
			matchTicket := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(t.Ticket)
			log.Infof("writing match ticket: %s", logJSONFormatter(matchTicket))
			ticketProvider.channelTickets <- matchTicket
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range resultChan {
			log.Info("crafting a MatchResponse")
			resp := matchfunctiongrpc.MatchResponse{Match: matchfunctiongrpc.MatchfunctionMatchToProtoMatch(result)}
			log.Infof("Response: %s", logJSONFormatter(resp))
			log.Infof("match made and being sent back to the client: %+v", &resp)
			if err := server.Send(&resp); err != nil {
				log.WithError(err).Errorf("error on server send")

				return
			}
			matchesMade++
		}
	}()
	wg.Wait()

	log.Infof("make matches finished and %d matches were made", matchesMade)

	return nil
}

// BackfillMatches uses the assigned MatchMaker to run backfill
func (m *MatchFunctionServer) BackfillMatches(server matchfunctiongrpc.MatchFunction_BackfillMatchesServer) error {
	ctx := server.Context()
	defer ctx.Done()

	log := logrus.WithField("method", "GRPC_SERVICE.BackfillMatches")

	log.Info("backfill matches")

	in, err := server.Recv()
	if err == io.EOF {
		log.Debug("Recv ended")
		return nil
	}
	if err != nil {
		log.WithError(err).Error("Recv error")
		return err
	}

	mrpT, ok := in.GetRequestType().(*matchfunctiongrpc.BackfillMakeMatchesRequest_Parameters)
	if !ok {
		log.Error("not a BackfillMakeMatchesRequest_Parameters type")

		return errors.New("expected parameters in the first message were not met")
	}

	rules, err := m.MM.RulesFromJSON(mrpT.Parameters.Rules.Json)
	if err != nil {
		log.WithError(err).Errorf("could not get rules from json")

		return err
	}

	log.WithField("rules", logJSONFormatter(rules)).Infof("Retrieved rules")

	ticketProvider := newMatchTicketProvider()

	go m.fetchBackfillTickets(ticketProvider, server)

	backfillProposal := m.MM.BackfillMatches(ticketProvider, rules)
	for {
		proposal, ok := <-backfillProposal
		if !ok {
			log.Info("no more proposal")
			return nil
		}

		resp := matchfunctiongrpc.BackfillResponse{
			BackfillProposal: matchfunctiongrpc.MatchfunctionBackfillProposalToProtoBackfillProposal(proposal),
		}

		log.WithField("proposal", logJSONFormatter(proposal)).Info("send proposal")

		err = server.Send(&resp)
		if err != nil {
			log.WithError(err).Error("send proposal error")
			return err
		}
	}
}

func (m *MatchFunctionServer) fetchBackfillTickets(ticketProvider matchTicketProvider, server matchfunctiongrpc.MatchFunction_BackfillMatchesServer) {
	log := logrus.WithField("method", "fetchBackfillTickets")

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
			log.WithError(err).Error("Recv error")
			return
		}

		if ticket := in.GetTicket(); ticket != nil {
			t := matchfunctiongrpc.ProtoTicketToMatchfunctionTicket(ticket)
			log.WithField("matchpool", t.MatchPool).
				WithField("ticketId", t.TicketID).Info("Received match ticket")
			ticketProvider.channelTickets <- t
		} else if backfillTicket := in.GetBackfillTicket(); backfillTicket != nil {
			t := matchfunctiongrpc.ProtoBackfillTicketToMatchfunctionBackfillTicket(backfillTicket)
			log.WithField("matchpool", t.MatchPool).
				WithField("ticketId", t.TicketID).Info("Received backfill ticket")
			ticketProvider.channelBackfillTickets <- t
		}
	}
}
