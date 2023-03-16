// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package matchmaker

import (
	"time"

	"matchmaking-function-grpc-plugin-server-go/pkg/player"
)

// Ticket represents a matchmaking request in a particular match pool for one or more players
type Ticket struct {
	Namespace        string
	PartySessionID   string
	TicketID         string
	MatchPool        string
	CreatedAt        time.Time
	Players          []player.PlayerData
	TicketAttributes map[string]interface{}
	Latencies        map[string]int64
}

// BackfillTicket represents a match result that needs additional players
type BackfillTicket struct {
	TicketID       string
	MatchPool      string
	CreatedAt      time.Time
	PartialMatch   Match
	MatchSessionID string
}

// BackfillProposal represents a proposal to update a match with additional players
type BackfillProposal struct {
	BackfillTicketID string
	CreatedAt        time.Time
	AddedTickets     []Ticket
	ProposedTeams    []Team
	ProposalID       string
	MatchPool        string
	MatchSessionID   string
}

// Team is a set of players that have been matched onto the same team
type Team struct {
	UserIDs []player.ID
}

// Match represents a matchmaking result with players placed on teams and tracking which tickets were included in the match
type Match struct {
	Tickets          []Ticket
	Teams            []Team
	RegionPreference []string // ordered list of
	MatchAttributes  map[string]interface{}
	Backfill         bool   // false for complete matches, true if more players are desired.
	ServerName       string // fill this with local DS name from ticket, used for directing match session to local DS
	ClientVersion    string // fill this with specific game version from ticket, for overriding DS version
}
