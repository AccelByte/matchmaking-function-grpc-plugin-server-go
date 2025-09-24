# Using AI to Build a Matchmaking Extend App and Generate `MakeMatches` Functions

## Introduction

Modern multiplayer games rely on matchmaking to create balanced, fair, and fun experiences.  
The AccelByte platform supports custom rules through a **Matchmaking Extend App**, where developers can override gRPC functions such as:

- `GetStatCodes`  
- `ValidateTicket`  
- `EnrichTicket`  
- `MakeMatches`  
- `BackfillMatches`  

Writing these functions by hand can be tedious. But with AI, you can speed up the process: AI can parse your **proto definitions**, read **rules.json**, and generate Go implementations that comply with the AccelByte contracts.

---

## Matchmaking Flow

From the provided proto and documentation:

1. **Client → Server**  
   - Send one `MakeMatchesRequest.parameters` (scope, rules.json, tickId)  
   - Send zero or more `MakeMatchesRequest.ticket` messages  
   - Close the stream  

2. **Server → Client**  
   - Emit zero or more `MatchResponse` objects  
   - Close the stream  

**Hard constraints**:  
- All tickets on a stream must belong to the same `match_pool`.  
- Discard tickets with 0 players or players exceeding `PlayersPerTeam` when Alliance is active.  
- Never split a ticket across teams.  

The **rules.json** drives behavior, typically containing:  
- **AllianceRule** – number of teams and players per team.  
- **MatchingRule** – attribute constraints.  
- **RegionLatencyMaxMs** – acceptable latency filters.  

---

## AI Workflow: Step by Step

Here’s a repeatable workflow to use AI for building a matchmaking extend app.

### Step 1. Provide the Proto

Upload or paste your `matchfunction.proto` into your AI assistant.  
This gives AI the structure of messages (`Ticket`, `Match`, `MakeMatchesRequest`, etc.) and RPC signatures.

### Step 2. Add Context with Rules JSON

Give AI a sample `rules.json`:

```json
{
  "AllianceRule": {
    "teams": 2,
    "playersPerTeam": 3
  }
}
```

This tells AI what constraints the implementation should enforce.

### Step 3. Prompt for Implementation

Ask AI:

> “Generate a Go `MakeMatches` implementation that enforces the Alliance rule: X teams × Y players per team.”

The AI will produce code that:  
- Parses `rules.json`  
- Collects tickets from the stream  
- Groups them into complete matches  
- Sends `MatchResponse` objects  

### Step 4. Insert Into Example Repo

Clone the [AccelByte reference repo](https://github.com/AccelByte/matchmaking-function-grpc-plugin-server-go):

```bash
git clone https://github.com/AccelByte/matchmaking-function-grpc-plugin-server-go
cd matchmaking-function-grpc-plugin-server-go
```

Replace the `MakeMatches` placeholder with your AI-generated function.  
Rebuild and run the server:

```bash
go mod tidy
go run main.go
```

### Step 5. Iterate with AI

- To add **region preferences**: prompt AI to “extend MakeMatches with lowest-latency region selection.”  
- To add **MMR balancing**: prompt AI to “distribute tickets evenly across teams by average MMR.”  
- To add **backfill support**: prompt AI to “generate BackfillMatches handler that accepts a PartialMatch.”  

AI will output updated code that you can drop in.

---

## Example: AI-Generated `MakeMatches` (Simplified)

```go
func (s *Server) MakeMatches(stream pb.MatchFunction_MakeMatchesServer) error {
    // Receive parameters
    first, _ := stream.Recv()
    params := first.GetParameters()
    alr, _ := parseAlliance(params.GetRules().GetJson())

    var candidates []*pb.Ticket
    for {
        req, err := stream.Recv()
        if err != nil { break }
        if t := req.GetTicket(); t != nil {
            if len(t.GetPlayers()) == 0 || len(t.GetPlayers()) > alr.PlayersPerTeam {
                continue
            }
            candidates = append(candidates, t)
        }
    }

    teams := groupIntoAlliance(candidates, alr)
    if teams == nil { return nil }

    match := buildMatch(params, teams)
    return stream.Send(&pb.MatchResponse{Match: match})
}
```

This version:  
- Validates tickets  
- Packs them into complete matches  
- Emits results following the proto contract  

---

## Why Use AI?

- **Speed**: Go from proto to working server in minutes.  
- **Correctness**: AI respects the streaming order and contract.  
- **Flexibility**: Quickly adapt to new rules (latency, MMR, cross-region).  
- **Iteration**: Each change is just a new prompt away.  

---

## Conclusion

By combining **AccelByte’s extensible matchmaking system** with **AI-assisted coding**, you can:  

- Generate working `MakeMatches` functions automatically  
- Encode Alliance rules and validation logic with minimal effort  
- Iterate rapidly on balancing strategies  

👉 Get started with the [AccelByte reference repo](https://github.com/AccelByte/matchmaking-function-grpc-plugin-server-go), then use AI to tailor the matchmaking logic for your game.  

---