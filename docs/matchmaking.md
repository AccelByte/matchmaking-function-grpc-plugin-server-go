# Category: Matchmaking

## Overview

This category defines matchmaking for the AccelByte platform. It handles make matches and backfill matches functionality.

### How to read this proto (flow guide for AI Agents)
- Typical lifecycle:
    1) GetStatCodes to derive which player stats/attributes you should fetch based on the Rules JSON.
    2) ValidateTicket to quickly reject tickets that will never match (e.g., no valid region latency).
    3) EnrichTicket to add or compute attributes in ticket_attributes before queuing.
    4) MakeMatches (bidirectional stream) to build new matches out of queued tickets.
    5) BackfillMatches (bidirectional stream) to add more tickets into ongoing sessions that are not yet full.
- Rules.json drives behavior for all of the above. The examples in assets/snippets use a RuleSet with AllianceRule (team sizes/count), MatchingRule (attribute-based constraints, e.g., distance from a pivot), and RegionLatencyMaxMs.
- Streaming contracts matter: for both MakeMatches and BackfillMatches, the first message on the stream must be the Parameters message; subsequent messages are the data items (tickets and/or backfill ticket). When the client stops sending, the server responds with zero or more responses on the same stream.

### High-level RPC summaries
- GetStatCodes: input Rules; output list of attribute names you should provide in Ticket.players[*].attributes for matching.
- ValidateTicket: checks a ticket against rules, commonly ensuring at least one region latency is within RegionLatencyMaxMs.
- EnrichTicket: mutates ticket_attributes to include computed values your matcher/backfill will later use.
- MakeMatches: client-streams parameters once, then many Ticket messages (same match_pool); server-streams Match objects when formed.
- BackfillMatches: client-streams parameters once, then the BackfillTicket (describing the partial session), then many Ticket messages; server-streams BackfillProposal objects when backfill is possible.

### Sequence details
- MakeMatches stream (client → server order):
    1) MakeMatchesRequest.parameters { scope, rules.json, tickId }
    2) MakeMatchesRequest.ticket repeated for each candidate ticket
    3) Client closes send; server may emit 0..N MatchResponse messages, each with a Match containing tickets, teams, region_preferences, match_attributes, and server_pool if provided.
- BackfillMatches stream (client → server order):
    1) BackfillMakeMatchesRequest.parameters { scope, rules.json, tickId }
    2) BackfillMakeMatchesRequest.backfill_ticket once, describing the ongoing session (PartialMatch)
    3) BackfillMakeMatchesRequest.ticket repeated for each candidate ticket to consider
    4) Client closes send; server may emit 0..N BackfillResponse messages, each with a BackfillProposal listing added_tickets and proposed_teams for the session.

### MakeMatches stream contract (normative)
State machine:
1. Client sends exactly **one** `MakeMatchesRequest.parameter`.
2. Client sends **N ≥ 0** `MakeMatchesRequest.ticket` messages.
3. Client half-closes the stream.
4. Server may send **0..M** `MatchResponse` messages, then closes.

Hard constraints:
- All `ticket.match_pool` on a given stream **must be identical to each other**.
- The server **must** discard any ticket whose `players.length == 0` or `players.length > PlayersPerTeam` when Alliance is active.
- The server **must not** split a ticket across different teams in the same match.

### Important message relationships
- Rules.json example:
    - AllianceRule: Min/MaxNumber of teams and PlayerMin/MaxNumber per team.
    - MatchingRule: attribute constraints (e.g., distance from a pivot average).
    - RegionLatencyMaxMs: filters tickets by acceptable latency for preferred regions.
- Ticket:
    - players[*].attributes is where stat codes live (e.g., MMR/ELO). Matching may average these across players in a ticket.
    - latencies map[string]int64 provides region → ms; per ticket; can be used to filter by region preferences or RegionLatencyMaxMs.
    - party_session_id groups users into a Party which must be placed together on one team.
- Match vs BackfillTicket.PartialMatch:
    - Match is the full output of MakeMatches.
    - BackfillTicket.PartialMatch represents the current, not‑yet‑full state of a session; BackfillProposal suggests how to update it with added_tickets and proposed_teams.

### Minimal working examples (pseudo)
- GetStatCodes:
    - Request: { rules.json }
    - Response: { codes: ["mmr", "skill"] }
- ValidateTicket:
    - Request: { ticket, rules.json }
    - Response: { valid_ticket: true|false }
- MakeMatches (stream):
    - C→S: parameters, ticket, ticket, ..., close
    - S→C: match, match, ...
- BackfillMatches (stream):
    - C→S: parameters, backfill_ticket, ticket, ticket, ..., close
    - S→C: backfill_proposal, backfill_proposal, ...
