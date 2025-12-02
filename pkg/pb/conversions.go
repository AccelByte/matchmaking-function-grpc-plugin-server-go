// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package matchfunction

import (
	"encoding/json"
	"matchmaking-function-grpc-plugin-server-go/pkg/matchmaker"
	"matchmaking-function-grpc-plugin-server-go/pkg/playerdata"

	"github.com/elliotchance/pie/v2"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MatchfunctionTicketToProtoTicket(ticket matchmaker.Ticket) *Ticket {
	log := logrus.WithField("function", "MatchfunctionTicketToProtoTicket")

	// convert ticket attributes to proto struct
	var err error
	ticket.TicketAttributes, err = convertAttribute(ticket.TicketAttributes)
	if err != nil {
		log.Errorf("value ticket attributes marshal : %v error : %s", ticket.TicketAttributes, err.Error())
		log.Errorf("error on convertAttribute for ticket attributes")
	}

	ticketAttrs, err := structpb.NewStruct(ticket.TicketAttributes)
	if err != nil {
		log.Errorf("value ticket attributes : %v error : %s", ticket.TicketAttributes, err.Error())
		log.Errorf("error on structpb for ticket attributes")
	}

	return &Ticket{
		state:         protoimpl.MessageState{},
		sizeCache:     0,
		unknownFields: nil,
		TicketId:      ticket.TicketID,
		MatchPool:     ticket.MatchPool,
		CreatedAt:     timestamppb.New(ticket.CreatedAt),
		Players: pie.Map(ticket.Players, func(p playerdata.PlayerData) *Ticket_PlayerData {
			p.Attributes, err = convertAttribute(p.Attributes)
			if err != nil {
				log.Errorf("value player attributes marshal : %v error : %s", p.Attributes, err.Error())
				log.Errorf("error on convertAttribute for player attributes")
			}
			playerAttrs, paErr := structpb.NewStruct(p.Attributes)
			if paErr != nil {
				log.Errorf("value player attributes : %v error : %s", p.Attributes, paErr.Error())
				log.Errorf("failed to create new proto struct for player attributes")
			}
			return &Ticket_PlayerData{
				PlayerId:   playerdata.IDToString(p.PlayerID),
				Attributes: playerAttrs,
			}
		}),
		TicketAttributes: ticketAttrs,
		Latencies:        ticket.Latencies,
		PartySessionId:   ticket.PartySessionID,
		Namespace:        ticket.Namespace,
		ExcludedSessions: ticket.ExcludedSessions,
	}
}

func MatchfunctionTicketsToProtoTickets(tickets []matchmaker.Ticket) []*Ticket {
	var result []*Ticket

	for _, ticket := range tickets {
		result = append(result, MatchfunctionTicketToProtoTicket(ticket))
	}

	return result
}

func ProtoTicketToMatchfunctionTicket(ticket *Ticket) matchmaker.Ticket {
	return matchmaker.Ticket{
		TicketID:  ticket.TicketId,
		MatchPool: ticket.MatchPool,
		CreatedAt: ticket.CreatedAt.AsTime(),
		Players: pie.Map(ticket.Players, func(p *Ticket_PlayerData) playerdata.PlayerData {
			return playerdata.PlayerData{
				PlayerID:   playerdata.IDFromString(p.PlayerId),
				Attributes: p.Attributes.AsMap(),
			}
		}),
		TicketAttributes: ticket.TicketAttributes.AsMap(),
		Latencies:        ticket.Latencies,
		PartySessionID:   ticket.PartySessionId,
		Namespace:        ticket.Namespace,
		ExcludedSessions: ticket.ExcludedSessions,
	}
}

func ProtoBackfillTicketToMatchfunctionBackfillTicket(ticket *BackfillTicket) matchmaker.BackfillTicket {
	return matchmaker.BackfillTicket{
		TicketID:       ticket.TicketId,
		MatchPool:      ticket.MatchPool,
		CreatedAt:      ticket.CreatedAt.AsTime(),
		PartialMatch:   ProtoPartialMatchToMatchfunctionMatch(ticket.PartialMatch),
		MatchSessionID: ticket.MatchSessionId,
	}
}

func protoBackfillTicketTeamToMatch(protoTeams []*BackfillTicket_Team) []matchmaker.Team {
	var teams []matchmaker.Team
	for _, protoTeam := range protoTeams {
		team := matchmaker.Team{
			TeamID: protoTeam.TeamId,
			UserIDs: pie.Map(protoTeam.UserIds, func(s string) playerdata.ID {
				return playerdata.IDFromString(s)
			}),
			Parties: pie.Map(protoTeam.Parties, func(party *Party) matchmaker.Party {
				return matchmaker.Party{
					UserIDs: party.UserIds,
					PartyID: party.PartyId,
				}
			}),
		}
		teams = append(teams, team)
	}

	return teams
}

func ProtoPartialMatchToMatchfunctionMatch(match *BackfillTicket_PartialMatch) matchmaker.Match {
	return matchmaker.Match{
		Tickets: pie.Map(match.Tickets, func(m *Ticket) matchmaker.Ticket {
			return matchmaker.Ticket{
				TicketID:  m.TicketId,
				MatchPool: m.MatchPool,
				CreatedAt: m.CreatedAt.AsTime(),
				Players: pie.Map(m.Players, func(p *Ticket_PlayerData) playerdata.PlayerData {
					return playerdata.PlayerData{PlayerID: playerdata.IDFromString(p.PlayerId), Attributes: p.Attributes.AsMap()}
				}),
				TicketAttributes: m.TicketAttributes.AsMap(),
				Latencies:        m.Latencies,
				PartySessionID:   m.PartySessionId,
				Namespace:        m.Namespace,
				ExcludedSessions: m.ExcludedSessions,
			}
		}),
		Teams:            protoBackfillTicketTeamToMatch(match.Teams),
		RegionPreference: match.RegionPreferences,
		MatchAttributes:  match.MatchAttributes.AsMap(),
		Backfill:         match.Backfill,
		ServerName:       match.ServerName,
		ClientVersion:    match.ClientVersion,
	}
}

func ProtoMatchToMatchfunctionMatch(match *Match) matchmaker.Match {
	var serverPool matchmaker.ServerPoolSelectionParameter
	if match.ServerPool != nil {
		serverPool = matchmaker.ServerPoolSelectionParameter{
			Deployment:     match.ServerPool.Deployment,
			ServerProvider: match.ServerPool.ServerProvider,
			ClaimKeys:      match.ServerPool.ClaimKeys,
		}
	}

	return matchmaker.Match{
		Tickets: pie.Map(match.Tickets, func(m *Ticket) matchmaker.Ticket {
			return matchmaker.Ticket{
				TicketID:  m.TicketId,
				MatchPool: m.MatchPool,
				CreatedAt: m.CreatedAt.AsTime(),
				Players: pie.Map(m.Players, func(p *Ticket_PlayerData) playerdata.PlayerData {
					return playerdata.PlayerData{PlayerID: playerdata.IDFromString(p.PlayerId), Attributes: p.Attributes.AsMap()}
				}),
				TicketAttributes: m.TicketAttributes.AsMap(),
				Latencies:        m.Latencies,
				PartySessionID:   m.PartySessionId,
				Namespace:        m.Namespace,
				ExcludedSessions: m.ExcludedSessions,
			}
		}),
		Teams:                        protoMatchTeamToMatch(match.Teams),
		RegionPreference:             match.RegionPreferences,
		MatchAttributes:              match.MatchAttributes.AsMap(),
		Backfill:                     match.Backfill,
		ServerName:                   match.ServerName,
		ClientVersion:                match.ClientVersion,
		ServerPoolSelectionParameter: serverPool,
	}
}

func protoMatchTeamToMatch(protoTeams []*Match_Team) []matchmaker.Team {
	var teams []matchmaker.Team
	for _, protoTeam := range protoTeams {
		team := matchmaker.Team{
			TeamID: protoTeam.TeamId,
			UserIDs: pie.Map(protoTeam.UserIds, func(s string) playerdata.ID {
				return playerdata.IDFromString(s)
			}),
			Parties: pie.Map(protoTeam.Parties, func(party *Party) matchmaker.Party {
				return matchmaker.Party{
					UserIDs: party.UserIds,
					PartyID: party.PartyId,
				}
			}),
		}
		teams = append(teams, team)
	}

	return teams
}

func MatchfunctionMatchToProtoMatch(match matchmaker.Match) *Match {
	log := logrus.WithField("function", "MatchfunctionMatchToProtoMatch")

	var err error
	match.MatchAttributes, err = convertAttribute(match.MatchAttributes)
	if err != nil {
		log.Errorf("value match attributes marshal : %v error : %s", match.MatchAttributes, err.Error())
		log.Errorf("error on convertAttribute for match attributes")
	}
	matchAttrs, mErr := structpb.NewStruct(match.MatchAttributes)
	if mErr != nil {
		log.Errorf("value match attributes : %v error : %s", match.MatchAttributes, mErr.Error())
		log.Errorf("error on structpb for match attributes")
	}
	return &Match{
		Tickets: pie.Map(match.Tickets, func(ticket matchmaker.Ticket) *Ticket {
			return MatchfunctionTicketToProtoTicket(ticket)
		}),
		Teams: pie.Map(match.Teams, func(team matchmaker.Team) *Match_Team {
			return &Match_Team{
				TeamId: team.TeamID,
				UserIds: pie.Map(team.UserIDs, func(x playerdata.ID) string {
					return playerdata.IDToString(x)
				}),
				Parties: pie.Map(team.Parties, func(p matchmaker.Party) *Party {
					return &Party{
						PartyId: p.PartyID,
						UserIds: p.UserIDs,
					}
				}),
			}
		}),
		RegionPreferences: match.RegionPreference,
		MatchAttributes:   matchAttrs,
		Backfill:          match.Backfill,
		ServerName:        match.ServerName,
		ClientVersion:     match.ClientVersion,
		ServerPool: &ServerPool{
			ServerProvider: match.ServerPoolSelectionParameter.ServerProvider,
			Deployment:     match.ServerPoolSelectionParameter.Deployment,
			ClaimKeys:      match.ServerPoolSelectionParameter.ClaimKeys,
		},
	}
}

// TODO: update to consider userID and partyID relationship.
func ProtoBackfillProposalToMatchfunctionBackfillProposal(match *BackfillProposal) matchmaker.BackfillProposal {
	return matchmaker.BackfillProposal{
		BackfillTicketID: match.BackfillTicketId,
		CreatedAt:        match.CreatedAt.AsTime(),
		AddedTickets: pie.Map(match.AddedTickets, func(m *Ticket) matchmaker.Ticket {
			return matchmaker.Ticket{
				TicketID:  m.TicketId,
				MatchPool: m.MatchPool,
				CreatedAt: m.CreatedAt.AsTime(),
				Players: pie.Map(m.Players, func(p *Ticket_PlayerData) playerdata.PlayerData {
					return playerdata.PlayerData{PlayerID: playerdata.IDFromString(p.PlayerId), Attributes: p.Attributes.AsMap()}
				}),
				TicketAttributes: m.TicketAttributes.AsMap(),
				Latencies:        m.Latencies,
				PartySessionID:   m.PartySessionId,
				Namespace:        m.Namespace,
				ExcludedSessions: m.ExcludedSessions,
			}
		}),
		ProposedTeams: pie.Map(match.ProposedTeams, func(team *BackfillProposal_Team) matchmaker.Team {
			return matchmaker.Team{
				TeamID: team.TeamId,
				UserIDs: pie.Map(team.UserIds, func(p string) playerdata.ID {
					return playerdata.ID(p)
				}),
				Parties: pie.Map(team.Parties, func(p *Party) matchmaker.Party {
					return matchmaker.Party{
						PartyID: p.PartyId,
						UserIDs: p.UserIds,
					}
				}),
			}
		}),
		MatchPool:      match.MatchPool,
		ProposalID:     match.ProposalId,
		MatchSessionID: match.MatchSessionId,
	}
}

func MatchfunctionBackfillTicketToProtoBackfillTicket(backfillTicket matchmaker.BackfillTicket) *BackfillTicket {
	log := logrus.WithField("function", "MatchfunctionBackfillTicketToProtoBackfillTicket")
	match := backfillTicket.PartialMatch
	var err error
	match.MatchAttributes, err = convertAttribute(match.MatchAttributes)
	if err != nil {
		log.Errorf("value match attributes marshal : %v error : %s", match.MatchAttributes, err.Error())
		log.Errorf("error on convertAttribute for match attributes")
	}
	// convert ticket attributes to proto struct
	ticketAttrs, err := structpb.NewStruct(match.MatchAttributes)
	if err != nil {
		log.Errorf("value match attributes : %v error : %s", match.MatchAttributes, err.Error())
		log.Errorf("error on structpb for ticket attributes")
	}
	var backfillTeams []*BackfillTicket_Team
	for _, team := range match.Teams {
		userIDs := pie.Map(team.UserIDs, func(p playerdata.ID) string {
			return playerdata.IDToString(p)
		})
		if len(userIDs) > 0 {
			backfillTeams = append(backfillTeams, &BackfillTicket_Team{
				TeamId:  team.TeamID,
				UserIds: userIDs,
				Parties: pie.Map(team.Parties, func(p matchmaker.Party) *Party {
					return &Party{
						PartyId: p.PartyID,
						UserIds: p.UserIDs,
					}
				}),
			})
		}
	}
	tickets := pie.Map(match.Tickets, func(t matchmaker.Ticket) *Ticket {
		t.TicketAttributes, err = convertAttribute(t.TicketAttributes)
		if err != nil {
			log.Errorf("value ticket attributes marshal : %v error : %s", t.TicketAttributes, err.Error())
			log.Errorf("error on convert Attribute ticket attributes")
		}

		attributes, err := structpb.NewStruct(t.TicketAttributes)
		if err != nil {
			log.Errorf("value ticket attributes : %v error : %s", t.TicketAttributes, err.Error())
			log.Errorf("error on structpb for ticket attributes")
		}
		playerData := pie.Map(t.Players, func(p playerdata.PlayerData) *Ticket_PlayerData {
			var err error
			p.Attributes, err = convertAttribute(p.Attributes)
			if err != nil {
				log.WithField("function", "MatchfunctionBackfillTicketToProtoBackfillTicket").Errorf("value player attributes marshal : %v error : %s", p.Attributes, err.Error())
				log.WithField("function", "MatchfunctionBackfillTicketToProtoBackfillTicket").Errorf("error on convertAttribute for player attributes")
			}
			playerAttrs, paErr := structpb.NewStruct(p.Attributes)
			if paErr != nil {
				logrus.Errorf("value player attributes : %v error : %s", p.Attributes, paErr.Error())
				logrus.Errorf("failed to create new proto struct for player attributes")
			}
			return &Ticket_PlayerData{
				state:      protoimpl.MessageState{},
				sizeCache:  0,
				PlayerId:   playerdata.IDToString(p.PlayerID),
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
			ExcludedSessions: t.ExcludedSessions,
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

func PlayerDataToParties(players []playerdata.PlayerData) []matchmaker.Party {
	mapParty := make(map[string][]string)

	for _, player := range players {
		mapParty[player.PartyID] = append(mapParty[player.PartyID], string(player.PlayerID))
	}

	parties := make([]matchmaker.Party, 0, len(mapParty))
	for partyID, userIDs := range mapParty {
		parties = append(parties, matchmaker.Party{
			PartyID: partyID,
			UserIDs: userIDs,
		})
	}
	return parties
}

func toProtoPlayerData(p playerdata.PlayerData) *Ticket_PlayerData {
	playerAttrs, paErr := structpb.NewStruct(p.Attributes)
	if paErr != nil {
		logrus.Errorf("failed to create new proto struct for playerdata attributes")
	}

	return &Ticket_PlayerData{
		state:      protoimpl.MessageState{},
		sizeCache:  0,
		PlayerId:   playerdata.IDToString(p.PlayerID),
		Attributes: playerAttrs,
	}
}

// ProtoBackfillProposalToMatchfunctionBackfillProposal will convert a proto backfill proposal to a matchmaker backfill proposal.
func MatchfunctionBackfillProposalToProtoBackfillProposal(match matchmaker.BackfillProposal) *BackfillProposal {
	log := logrus.WithField("function", "MatchfunctionBackfillProposalToProtoBackfillProposal")

	pbAttributes, err := structpb.NewStruct(match.Attributes)
	if err != nil {
		log.Errorf("value match attributes : %v error : %s", match.Attributes, err.Error())
		log.Errorf("error on structpb for match attributes")
	}
	team := []*BackfillProposal_Team{}
	for _, data := range match.ProposedTeams {
		users := []string{}
		for _, user := range data.UserIDs {
			users = append(users, string(user))
		}

		parties := []*Party{}
		for _, party := range data.Parties {
			parties = append(parties, &Party{
				PartyId: party.PartyID,
				UserIds: party.UserIDs,
			})
		}

		team = append(team, &BackfillProposal_Team{
			UserIds: users,
			Parties: parties,
			TeamId:  data.TeamID,
		})
	}

	return &BackfillProposal{
		BackfillTicketId: match.BackfillTicketID,
		CreatedAt:        timestamppb.New(match.CreatedAt),
		AddedTickets: pie.Map(match.AddedTickets, func(t matchmaker.Ticket) *Ticket {
			pbTicketAttributes, err := structpb.NewStruct(t.TicketAttributes)
			if err != nil {
				log.Errorf("value match attributes : %v error : %s", match.Attributes, err.Error())
				log.Errorf("error on structpb for match attributes")
			}
			return &Ticket{
				TicketId:         t.TicketID,
				MatchPool:        t.MatchPool,
				CreatedAt:        timestamppb.New(t.CreatedAt),
				Players:          pie.Map(t.Players, toProtoPlayerData),
				TicketAttributes: pbTicketAttributes,
				Latencies:        t.Latencies,
				PartySessionId:   t.PartySessionID,
				Namespace:        t.Namespace,
				ExcludedSessions: t.ExcludedSessions,
			}
		}),
		ProposedTeams:  team,
		ProposalId:     match.ProposalID,
		MatchPool:      match.MatchPool,
		MatchSessionId: match.MatchSessionID,
		Attributes:     pbAttributes,
	}
}

func convertAttribute(data map[string]interface{}) (map[string]interface{}, error) {
	marshal, err := json.Marshal(data)
	if err != nil {
		return data, err
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(marshal, &result)
	if err != nil {
		return data, err
	}

	return result, err
}
