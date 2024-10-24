#!/usr/bin/env bash

# Matchmaking function demo script to simulate the matchmaking flow which calls this sample gRPC server

# Requires: bash curl jq

set -e
set -o pipefail

test -n "$AB_CLIENT_ID" || (echo "AB_CLIENT_ID is not set"; exit 1)
test -n "$AB_CLIENT_SECRET" || (echo "AB_CLIENT_SECRET is not set"; exit 1)
test -n "$AB_NAMESPACE" || (echo "AB_NAMESPACE is not set"; exit 1)

if [ -z "$GRPC_SERVER_URL" ] && [ -z "$EXTEND_APP_NAME" ]; then
  echo "GRPC_SERVER_URL or EXTEND_APP_NAME is not set"
  exit 1
fi

DEMO_PREFIX='mmv2_grpc_demo'
NUMBER_OF_PLAYERS=3
PLAYER_USER_ID_LIST=()

api_curl()
{
  curl -s -D api_curl_http_header.out -o api_curl_http_response.out -w '%{http_code}' "$@" > api_curl_http_code.out
  echo >> api_curl_http_response.out
  cat api_curl_http_response.out
}

get_code_verifier() 
{
  echo $RANDOM | sha256sum | cut -d ' ' -f 1   # For demo only: In reality, it needs to be secure random
}

get_code_challenge()
{
  echo -n "$1" | sha256sum | xxd -r -p | base64 | tr -d '\n' | sed -e 's/+/-/g' -e 's/\//\_/g' -e 's/=//g'
}

function clean_up()
{
  echo Getting player ...

  for PLAYER_NUMBER in $(seq $NUMBER_OF_PLAYERS); do
    echo Getting existing player $PLAYER_NUMBER ${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com ...

    USER_ID=$(api_curl -X GET "${AB_BASE_URL}/iam/v3/admin/namespaces/$AB_NAMESPACE/users?emailAddress=${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com" -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' | jq --raw-output .userId)

    PLAYER_USER_ID_LIST+=( $USER_ID )

    # shellcheck disable=SC2068
    for USER_ID in ${PLAYER_USER_ID_LIST[@]}; do
      echo Clean up existing player UserId: $USER_ID ...

      api_curl -X DELETE "${AB_BASE_URL}/iam/v3/admin/namespaces/$AB_NAMESPACE/users/$USER_ID/information" \
          -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null   # Ignore delete error
    done
  done

  echo Clean up match pool ...

  api_curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-pools/${DEMO_PREFIX}_pool" \
      -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null       # Ignore delete error

  echo Clean up rule sets ...

  api_curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/rulesets/${DEMO_PREFIX}_ruleset" \
      -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null       # Ignore delete error

  echo Clean up session template ...

  api_curl -X DELETE "${AB_BASE_URL}/session/v1/admin/namespaces/$AB_NAMESPACE/configurations/${DEMO_PREFIX}_template" \
      -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null       # Ignore delete error
}

echo Logging in client ...

ACCESS_TOKEN="$(api_curl ${AB_BASE_URL}/iam/v3/oauth/token \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -u "$AB_CLIENT_ID:$AB_CLIENT_SECRET" \
    -d "grant_type=client_credentials" | jq --raw-output .access_token)"

if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
  cat api_curl_http_response.out
  exit 1
fi

clean_up

trap clean_up EXIT

echo Creating session template ...

api_curl "${AB_BASE_URL}/session/v1/admin/namespaces/$AB_NAMESPACE/configuration" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"clientVersion\":\"1.0.0\",\"deployment\":null,\"inactiveTimeout\":60,\"inviteTimeout\":60,\"joinability\":\"OPEN\",\"maxPlayers\":2,\"minPlayers\":2,\"name\":\"${DEMO_PREFIX}_template\",\"requestedRegions\":[\"us-west-2\"],\"textChat\":null,\"type\":\"P2P\"}"

if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
  exit 1
fi

echo Creating rule sets ...

api_curl "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/rulesets" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H 'Content-Type: application/json' \
    -d "{\"enable_custom_match_function\":true,\"data\":{\"shipCountMin\":2,\"shipCountMax\":2},\"name\":\"${DEMO_PREFIX}_ruleset\"}"

if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
  exit 1
fi

if [ -n "$GRPC_SERVER_URL" ]; then
  echo Registering match function URL \(replace exising\) $GRPC_SERVER_URL ...

  api_curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions/${DEMO_PREFIX}_function" \
      -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null     # Ignore delete error for non-existing match function

  api_curl "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions" \
      -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
      -d "{\"match_function\":\"${DEMO_PREFIX}_function\",\"url\":\"$GRPC_SERVER_URL\"}"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    exit 1
  fi
elif [ -n "$EXTEND_APP_NAME" ]; then
  echo Registering match function name \(replace exising\) $EXTEND_APP_NAME ...

  api_curl -X DELETE "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions/${DEMO_PREFIX}_function" \
      -H "Authorization: Bearer $ACCESS_TOKEN" >/dev/null     # Ignore delete error for non-existing match function

  api_curl "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-functions" \
      -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
      -d "{\"match_function\":\"${DEMO_PREFIX}_function\",\"serviceAppName\":\"$EXTEND_APP_NAME\"}"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    exit 1
  fi
else
  echo "GRPC_SERVER_URL or EXTEND_APP_NAME is not set"
  exit 1
fi

echo Creating match pool ...

api_curl "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-pools" \
    -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
    -d "{\"backfill_ticket_expiration_seconds\":600,\"match_function\":\"${DEMO_PREFIX}_function\",\"name\":\"${DEMO_PREFIX}_pool\",\"rule_set\":\"${DEMO_PREFIX}_ruleset\",\"session_template\":\"${DEMO_PREFIX}_template\",\"ticket_expiration_seconds\":600}"

if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
  exit 1
fi

for PLAYER_NUMBER in $(seq $NUMBER_OF_PLAYERS); do
  echo Creating player $PLAYER_NUMBER ${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com ...

  USER_ID="$(api_curl "${AB_BASE_URL}/iam/v4/public/namespaces/$AB_NAMESPACE/users" \
      -H "Authorization: Bearer $ACCESS_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "{\"authType\":\"EMAILPASSWD\",\"country\":\"ID\",\"dateOfBirth\":\"1995-01-10\",\"displayName\":\"MMv2 gRPC Player\",\"uniqueDisplayName\":\"MMv2 gRPC Player$PLAYER_NUMBER\",\"emailAddress\":\"${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com\",\"password\":\"GFPPlmdb2-\",\"username\":\"${DEMO_PREFIX}_player_$PLAYER_NUMBER\"}" | jq --raw-output .userId)"
  
  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    cat api_curl_http_response.out
    exit 1
  fi

  PLAYER_USER_ID_LIST+=( $USER_ID )
  
  echo Logging in player $PLAYER_NUMBER ...
  
  CODE_VERIFIER="$(get_code_verifier)"

  api_curl "${AB_BASE_URL}/iam/v3/oauth/authorize?scope=commerce+account+social+publishing+analytics&response_type=code&code_challenge_method=S256&code_challenge=$(get_code_challenge "$CODE_VERIFIER")&client_id=$AB_CLIENT_ID"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    exit 1
  fi

  REQUEST_ID="$(cat api_curl_http_header.out | grep -o 'request_id=[a-f0-9]\+' | cut -d= -f2)"
  
  api_curl ${AB_BASE_URL}/iam/v3/authenticate \
      -H 'Content-Type: application/x-www-form-urlencoded' \
      -d "password=GFPPlmdb2-&user_name=${DEMO_PREFIX}_player_$PLAYER_NUMBER@test.com&request_id=$REQUEST_ID&client_id=$AB_CLIENT_ID"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    exit 1
  fi

  CODE="$(cat api_curl_http_header.out | grep -o 'code=[a-f0-9]\+' | cut -d= -f2)"
  
  PLAYER_ACCESS_TOKEN="$(api_curl ${AB_BASE_URL}/iam/v3/oauth/token \
      -H 'Content-Type: application/x-www-form-urlencoded' -u "$AB_CLIENT_ID:$AB_CLIENT_SECRET" \
      -d "code=$CODE&grant_type=authorization_code&client_id=$AB_CLIENT_ID&code_verifier=$CODE_VERIFIER" | jq --raw-output .access_token)"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    cat api_curl_http_response.out
    exit 1
  fi

  echo Creating player $PLAYER_NUMBER match ticket ...
  
  MATCH_TICKET_ID="$(api_curl "${AB_BASE_URL}/match2/v1/namespaces/$AB_NAMESPACE/match-tickets" \
      -H "Authorization: Bearer $PLAYER_ACCESS_TOKEN" \
      -H 'Content-Type: application/json' \
      -d "{\"attributes\":null,\"latencies\":null,\"matchPool\":\"${DEMO_PREFIX}_pool\",\"sessionID\":\"\"}" | jq --raw-output .matchTicketID)"

  if [ "$(cat api_curl_http_code.out)" -ge "400" ]; then
    cat api_curl_http_response.out
    exit 1
  fi

  echo Player $PLAYER_NUMBER UserId: $USER_ID, MatchTicketId: $MATCH_TICKET_ID

  if ! echo -n "$MATCH_TICKET_ID" | grep -q '[0-9a-f]\+'; then
    echo "Failed! Not getting the expected MatchTicketId."
    exit 1
  fi
done
