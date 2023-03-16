// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package matchfunction

import (
	pie_ "github.com/elliotchance/pie/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"matchmaking-function-grpc-plugin-server-go/pkg/player"
)

// MatchfunctionTicketToProtoTicket will convert a matchmaker ticket to a proto ticket
func MatchfunctionTicketToProtoTicket(ticket matchmaker.Ticket) *Ticket {
	// convert ticket attributes to proto struct
	ticketAttrs, err := structpb.NewStruct(ticket.TicketAttributes)
	if err != nil {
		logrus.Errorf("error on structpb for ticket attributes")
	}

	return &Ticket{
		state:         protoimpl.MessageState{},
		sizeCache:     0,
		unknownFields: nil,
		TicketId:      ticket.TicketID,
		MatchPool:     ticket.MatchPool,
		CreatedAt:     timestamppb.New(ticket.CreatedAt),
		Players: pie_.Map(ticket.Players, func(p player.PlayerData) *Ticket_PlayerData {
			playerAttrs, paErr := structpb.NewStruct(p.Attributes)
			if paErr != nil {
				logrus.Errorf("failed to create new proto struct for player attributes")
			}
			return &Ticket_PlayerData{
				state:      protoimpl.MessageState{},
				sizeCache:  0,
				PlayerId:   player.IDToString(p.PlayerID),
				Attributes: playerAttrs,
			}
		}),
		TicketAttributes: ticketAttrs,
		Latencies:        ticket.Latencies,
		PartySessionId:   ticket.PartySessionID,
		Namespace:        ticket.Namespace,
	}
}

// ProtoTicketToMatchfunctionTicket will convert a proto ticket to a matchmaker ticket
func ProtoTicketToMatchfunctionTicket(ticket *Ticket) matchmaker.Ticket {
	return matchmaker.Ticket{
		TicketID:  ticket.TicketId,
		MatchPool: ticket.MatchPool,
		CreatedAt: ticket.CreatedAt.AsTime(),
		Players: pie_.Map(ticket.Players, func(p *Ticket_PlayerData) player.PlayerData {
			return player.PlayerData{
				PlayerID:   player.IDFromString(p.PlayerId),
				Attributes: p.Attributes.AsMap(),
			}
		}),
		TicketAttributes: ticket.TicketAttributes.AsMap(),
		Latencies:        ticket.Latencies,
		PartySessionID:   ticket.PartySessionId,
		Namespace:        ticket.Namespace,
	}
}

// ProtoMatchToMatchfunctionMatch will convert a proto match to a matchmaker match
func ProtoMatchToMatchfunctionMatch(match *Match) matchmaker.Match {
	return matchmaker.Match{
		Tickets: pie_.Map(match.Tickets, func(m *Ticket) matchmaker.Ticket {
			return matchmaker.Ticket{
				TicketID:  m.TicketId,
				MatchPool: m.MatchPool,
				CreatedAt: m.CreatedAt.AsTime(),
				Players: pie_.Map(m.Players, func(p *Ticket_PlayerData) player.PlayerData {
					return player.PlayerData{PlayerID: player.IDFromString(p.PlayerId), Attributes: p.Attributes.AsMap()}
				}),
				TicketAttributes: m.TicketAttributes.AsMap(),
				Latencies:        m.Latencies,
				PartySessionID:   m.PartySessionId,
				Namespace:        m.Namespace,
			}
		}),
		Teams: pie_.Map(match.Teams, func(team *Match_Team) matchmaker.Team {
			return matchmaker.Team{UserIDs: pie_.Map(team.UserIds, func(p string) player.ID {
				return player.ID(p)
			})}
		}),
		RegionPreference: match.RegionPreferences,
		MatchAttributes:  match.MatchAttributes.AsMap(),
		Backfill:         match.Backfill,
		ServerName:       match.ServerName,
		ClientVersion:    match.ClientVersion,
	}
}

// MatchfunctionMatchToProtoMatch will conert a matchmaker match to a proto match
func MatchfunctionMatchToProtoMatch(match matchmaker.Match) *Match {
	matchAttrs, mErr := structpb.NewStruct(match.MatchAttributes)
	if mErr != nil {
		logrus.Errorf("error on structpb for match attributes")
	}
	return &Match{
		Tickets: pie_.Map(match.Tickets, func(ticket matchmaker.Ticket) *Ticket {
			return MatchfunctionTicketToProtoTicket(ticket)
		}),
		Teams: pie_.Map(match.Teams, func(team matchmaker.Team) *Match_Team {
			return &Match_Team{UserIds: pie_.Map(team.UserIDs, func(x player.ID) string {
				return player.IDToString(x)
			})}
		}),
		RegionPreferences: match.RegionPreference,
		MatchAttributes:   matchAttrs,
		Backfill:          match.Backfill,
		ServerName:        match.ServerName,
		ClientVersion:     match.ClientVersion,
	}
}

// TODO: update to consider userID and partyID relationship
// ProtoBackfillProposalToMatchfunctionBackfillProposal will convert a proto backfill proposal to a matchmaker backfill proposal
func ProtoBackfillProposalToMatchfunctionBackfillProposal(match *BackfillProposal) matchmaker.BackfillProposal {
	return matchmaker.BackfillProposal{
		BackfillTicketID: match.BackfillTicketId,
		CreatedAt:        match.CreatedAt.AsTime(),
		AddedTickets: pie_.Map(match.AddedTickets, func(m *Ticket) matchmaker.Ticket {
			return matchmaker.Ticket{
				TicketID:  m.TicketId,
				MatchPool: m.MatchPool,
				CreatedAt: m.CreatedAt.AsTime(),
				Players: pie_.Map(m.Players, func(p *Ticket_PlayerData) player.PlayerData {
					return player.PlayerData{PlayerID: player.IDFromString(p.PlayerId), Attributes: p.Attributes.AsMap()}
				}),
				TicketAttributes: m.TicketAttributes.AsMap(),
				Latencies:        m.Latencies,
				PartySessionID:   m.PartySessionId,
				Namespace:        m.Namespace,
			}
		}),
		ProposedTeams: pie_.Map(match.ProposedTeams, func(team *BackfillProposal_Team) matchmaker.Team {
			return matchmaker.Team{UserIDs: pie_.Map(team.UserIds, func(p string) player.ID {
				return player.ID(p)
			})}
		}),
		MatchPool:      match.MatchPool,
		ProposalID:     match.ProposalId,
		MatchSessionID: match.MatchSessionId,
	}
}

// MatchfunctionBackfillTicketToProtoBackfillTicket will convert a matchmaker backfill ticket to a proto backfill ticket
func MatchfunctionBackfillTicketToProtoBackfillTicket(backfillTicket matchmaker.BackfillTicket) *BackfillTicket {
	match := backfillTicket.PartialMatch
	// convert ticket attributes to proto struct
	ticketAttrs, err := structpb.NewStruct(match.MatchAttributes)
	if err != nil {
		logrus.Errorf("error on structpb for ticket attributes")
	}
	var backfillTeams []*BackfillTicket_Team
	for _, team := range match.Teams {
		userIDs := pie_.Map(team.UserIDs, func(p player.ID) string {
			return player.IDToString(p)
		})
		if len(userIDs) > 0 {
			backfillTeams = append(backfillTeams, &BackfillTicket_Team{UserIds: userIDs})
		}
	}
	tickets := pie_.Map(match.Tickets, func(t matchmaker.Ticket) *Ticket {
		attributes, err := structpb.NewStruct(t.TicketAttributes)
		if err != nil {
			logrus.Errorf("error on structpb for ticket attributes")
		}
		playerData := pie_.Map(t.Players, func(p player.PlayerData) *Ticket_PlayerData {
			playerAttrs, paErr := structpb.NewStruct(p.Attributes)
			if paErr != nil {
				logrus.Errorf("failed to create new proto struct for player attributes")
			}
			return &Ticket_PlayerData{
				state:      protoimpl.MessageState{},
				sizeCache:  0,
				PlayerId:   player.IDToString(p.PlayerID),
				Attributes: playerAttrs,
			}
		})
		return &Ticket{
			state:            protoimpl.MessageState{},
			sizeCache:        0,
			TicketId:         t.TicketID,
			MatchPool:        t.MatchPool,
			CreatedAt:        timestamppb.New(t.CreatedAt),
			Players:          playerData,
			TicketAttributes: attributes,
			Latencies:        t.Latencies,
			PartySessionId:   t.PartySessionID,
			Namespace:        t.Namespace,
		}
	})
	return &BackfillTicket{
		state:     protoimpl.MessageState{},
		sizeCache: 0,
		TicketId:  backfillTicket.TicketID,
		MatchPool: backfillTicket.MatchPool,
		CreatedAt: timestamppb.New(backfillTicket.CreatedAt),
		PartialMatch: &BackfillTicket_PartialMatch{
			state:             protoimpl.MessageState{},
			sizeCache:         0,
			Tickets:           tickets,
			Backfill:          match.Backfill,
			ServerName:        match.ServerName,
			ClientVersion:     match.ClientVersion,
			Teams:             backfillTeams,
			MatchAttributes:   ticketAttrs,
			RegionPreferences: match.RegionPreference,
		},
		MatchSessionId: backfillTicket.MatchSessionID,
	}
}
