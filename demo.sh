#!/usr/bin/env bash

# Matchmaking function demo script to simulate the matchmaking flow which calls this sample gRPC server

# Requires: bash curl jq

set -e
set -o pipefail

test -n "$NGROK_URL" || (echo "NGROK_URL is not set"; exit 1)
test -n "$AB_CLIENT_ID" || (echo "AB_CLIENT_ID is not set"; exit 1)
test -n "$AB_CLIENT_SECRET" || (echo "AB_CLIENT_SECRET is not set"; exit 1)
test -n "$AB_NAMESPACE" || (echo "AB_NAMESPACE is not set"; exit 1)

DEMO_PREFIX='mmv2_grpc_demo'
NUMBER_OF_PLAYERS=3

get_code_verifier() 
{
  echo $RANDOM | sha256sum | cut -d ' ' -f 1   # For demo only: In reality, it needs to be secure random
}

get_code_challenge()
{
  echo -n "$1" | sha256sum | xxd -r -p | base64 -w 0 | sed -e 's/\+/-/g' -e 's/\//\_/g' -e 's/=//g'
}

echo Logging in client ...

ACCESS_TOKEN="$(curl -s ${AB_BASE_URL}/iam/v3/oauth/token -H 'Content-Type: application/x-www-form-urlencoded' -u "$AB_CLIENT_ID:$AB_CLIENT_SECRET" -d "grant_type=client_credentials" | jq --raw-output .access_token)"

echo Creating session template ...

curl -s "${AB_BASE_URL}/session/v1/admin/namespaces/$AB_NAMESPACE/configuration" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"clientVersion\":\"1.0.0\",\"deployment\":null,\"inactiveTimeout\":60,\"inviteTimeout\":60,\"joinability\":\"OPEN\",\"maxPlayers\":2,\"minPlayers\":2,\"name\":\"${DEMO_PREFIX}_template\",\"requestedRegions\":[\"us-west-2\"],\"textChat\":null,\"type\":\"P2P\"}" > /dev/null

echo Creating rule sets ...

curl -s "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/rulesets" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"data\":{\"shipCountMin\":2,\"shipCountMax\":2},\"name\":\"${DEMO_PREFIX}_ruleset\"}"

echo Registering match function \(replace exising\) $NGROK_URL ...

curl -s -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions/${DEMO_PREFIX}_function" -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null

curl -s "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"match_function\":\"${DEMO_PREFIX}_function\",\"url\":\"$NGROK_URL\"}"

echo Creating match pool ...

curl -s "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-pools" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"backfill_ticket_expiration_seconds\":600,\"match_function\":\"${DEMO_PREFIX}_function\",\"name\":\"${DEMO_PREFIX}_pool\",\"rule_set\":\"${DEMO_PREFIX}_ruleset\",\"session_template\":\"${DEMO_PREFIX}_template\",\"ticket_expiration_seconds\":600}"

echo "Press ENTER to run the matchmaking flow"
read

for PLAYER_NUMBER in $(seq $NUMBER_OF_PLAYERS); do
  echo Creating player $PLAYER_NUMBER ...
  
  USER_ID="$(curl -s "${AB_BASE_URL}/iam/v4/public/namespaces/$AB_NAMESPACE/users" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"authType\":\"EMAILPASSWD\",\"country\":\"ID\",\"dateOfBirth\":\"1995-01-10\",\"displayName\":\"MMv2 gRPC Player\",\"emailAddress\":\"${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com\",\"password\":\"GFPPlmdb2-\",\"username\":\"${DEMO_PREFIX}_player_$PLAYER_NUMBER\"}" | jq --raw-output .userId)"
  
  if [ "$USER_ID" == "null" ]; then
    echo "Failed to create player with email ${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com, please delete existing first!"
    exit 1
  fi
  
  echo Logging in player $PLAYER_NUMBER ...
  
  CODE_VERIFIER="$(get_code_verifier)" 
  
  REQUEST_ID="$(curl -s -D - "${AB_BASE_URL}/iam/v3/oauth/authorize?scope=commerce+account+social+publishing+analytics&response_type=code&code_challenge_method=S256&code_challenge=$(get_code_challenge "$CODE_VERIFIER")&client_id=$AB_CLIENT_ID" | grep -o 'request_id=[a-f0-9]\+' | cut -d= -f2)"

  CODE="$(curl -s -D - ${AB_BASE_URL}/iam/v3/authenticate -H 'Content-Type: application/x-www-form-urlencoded' -d "password=GFPPlmdb2-&user_name=${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com&request_id=$REQUEST_ID&client_id=$AB_CLIENT_ID" | grep -o 'code=[a-f0-9]\+' | cut -d= -f2)"

  PLAYER_ACCESS_TOKEN="$(curl -s ${AB_BASE_URL}/iam/v3/oauth/token -H 'Content-Type: application/x-www-form-urlencoded' -u "$AB_CLIENT_ID:$AB_CLIENT_SECRET" -d "code=$CODE&grant_type=authorization_code&client_id=$AB_CLIENT_ID&code_verifier=$CODE_VERIFIER" | jq --raw-output .access_token)"

  echo Creating player $PLAYER_NUMBER match ticket ...
  
  MATCH_TICKET_ID="$(curl -s "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-tickets" -H "Authorization: Bearer $PLAYER_ACCESS_TOKEN" -H 'Content-Type: application/json' -d "{\"attributes\":null,\"latencies\":null,\"matchPool\":\"${DEMO_PREFIX}_pool\",\"sessionID\":\"\"}" | jq --raw-output .matchTicketID)"
  
  echo Player $PLAYER_NUMBER UserId: $USER_ID, MatchTicketId: $MATCH_TICKET_ID
    

  curl -X DELETE "${AB_BASE_URL}/iam/v3/admin/namespaces/$AB_NAMESPACE/users/$USER_ID/information" -H "Authorization: Bearer $ACCESS_TOKEN"  # For demo only: In reality, player is not supposed to be deleted immediately after creating match ticket
done

echo "Press ENTER to clean up"
read

echo Deleting match pool ...

curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-pools/${DEMO_PREFIX}_pool" -H "Authorization: Bearer $ACCESS_TOKEN"

echo Deleting rule sets ...

curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/rulesets/${DEMO_PREFIX}_ruleset" -H "Authorization: Bearer $ACCESS_TOKEN"

echo Deleting session template ...

curl -X DELETE "${AB_BASE_URL}/session/v1/admin/namespaces/$AB_NAMESPACE/configurations/${DEMO_PREFIX}_template" -H "Authorization: Bearer $ACCESS_TOKEN"