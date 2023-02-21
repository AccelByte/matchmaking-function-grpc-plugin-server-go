// Copyright (c) 2022 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/structpb"

	matchfunctiongrpc "matchmaking-function-grpc-plugin-server-go/pkg/pb"
)

// MatchFunctionServer is for the handler (upper level of match logic)
type MatchFunctionServer struct {
	matchfunctiongrpc.UnimplementedMatchFunctionServer

	shipCountMin     int
	shipCountMax     int
	unmatchedTickets []*matchfunctiongrpc.Ticket
}

func (m *MatchFunctionServer) GetStatCodes(ctx context.Context, req *matchfunctiongrpc.GetStatCodesRequest) (*matchfunctiongrpc.StatCodesResponse, error) {
	codes := []string{"2", "2"}
	logrus.Infof("stat codes: %s", codes)
	return &matchfunctiongrpc.StatCodesResponse{Codes: codes}, nil
}

func (m *MatchFunctionServer) ValidateTicket(ctx context.Context, req *matchfunctiongrpc.ValidateTicketRequest) (*matchfunctiongrpc.ValidateTicketResponse, error) {
	logrus.Info("validate ticket")
	return &matchfunctiongrpc.ValidateTicketResponse{ValidTicket: true}, nil
}

func (m *MatchFunctionServer) EnrichTicket(ctx context.Context, req *matchfunctiongrpc.EnrichTicketRequest) (*matchfunctiongrpc.EnrichTicketResponse, error) {
	logrus.Info("enrich ticket")
	// this will enrich ticket with these hardcoded ticket attributes
	enrichMap := map[string]*structpb.Value{
		"mmr":        structpb.NewNumberValue(250.0),
		"teamrating": structpb.NewNumberValue(2000.0),
	}

	if req.Ticket.TicketAttributes == nil || req.Ticket.TicketAttributes.Fields == nil {
		req.Ticket.TicketAttributes = &structpb.Struct{Fields: enrichMap}
	} else {
		for key, value := range enrichMap {
			req.Ticket.TicketAttributes.Fields[key] = value
		}
	}
	return &matchfunctiongrpc.EnrichTicketResponse{Ticket: req.Ticket}, nil
}

func (m *MatchFunctionServer) MakeMatches(server matchfunctiongrpc.MatchFunction_MakeMatchesServer) error {
	ctx := server.Context()
	defer ctx.Done()

	// set default gameRules value
	m.shipCountMax = 2
	m.shipCountMin = 2

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		in, err := server.Recv()
		if err == io.EOF {
			logrus.Infof("exit")
			return nil
		} else if err != nil {
			logrus.Errorf("error receiving from stream: %v", err)
			return err
		}
		if inParameters, isParameters := in.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Parameters); isParameters {
			ruleObject := &GameRules{}

			rulesJson := inParameters.Parameters.Rules.Json
			err = json.Unmarshal([]byte(rulesJson), ruleObject)

			newShipCountMin := ruleObject.ShipCountMin
			newShipCountMax := ruleObject.ShipCountMax
			if newShipCountMin != 0 &&
				newShipCountMax != 0 &&
				newShipCountMin <= newShipCountMax {
				m.shipCountMin = newShipCountMin
				m.shipCountMax = newShipCountMax
			}
			logrus.Infof("updated shipCountMin: %d shipCountMax: %d", m.shipCountMin, m.shipCountMax)
		} else if inTicket, isTicket := in.GetRequestType().(*matchfunctiongrpc.MakeMatchesRequest_Ticket); isTicket {
			m.unmatchedTickets = append(m.unmatchedTickets, inTicket.Ticket)
			if len(m.unmatchedTickets) == m.shipCountMax {
				userIds := make([]string, 0)
				for _, unmatchedTicket := range m.unmatchedTickets {
					for _, player := range unmatchedTicket.Players {
						userIds = append(userIds, player.PlayerId)
					}
				}

				matchResponse := &matchfunctiongrpc.MatchResponse{
					Match: &matchfunctiongrpc.Match{
						Teams: []*matchfunctiongrpc.Match_Team{
							{
								UserIds: userIds,
							},
						},
						RegionPreferences: []string{"any"},
						ClientVersion:     "v0.0.1",
					},
				}

				err = server.Send(matchResponse)
				if err != nil {
					logrus.Errorf("error sending to stream: %v", err)
					return err
				}

				logrus.Infof("created a match for: %v", proto.MarshalTextString(matchResponse))
				m.unmatchedTickets = make([]*matchfunctiongrpc.Ticket, 0)
			}
			logrus.Infof("unmatched ticket size: %d", len(m.unmatchedTickets))
		} else {
			return errors.New("invalid input")
		}

	}
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
