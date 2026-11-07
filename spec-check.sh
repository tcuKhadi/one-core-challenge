#!/usr/bin/env bash
# 1 Core Challenge - Specification & Durability Conformance Suite
# Verifies an entry implements api.md and satisfies rules.md crash durability.
#
# Usage:   ./spec-check.sh [container-name] [base-url]
# Example: ./spec-check.sh challenge http://localhost:8080
# Needs:   curl, jq, openssl, and docker (for durability crash check)

set -uo pipefail

CONTAINER="${1:-}"
BASE="${2:-http://localhost:8080}"
FAILS=0
RESP="$(mktemp)"
TS="$(date +%s)"
U1="u1_${TS}"
U2="u2_${TS}"

# Colors
C_RESET="\033[0m"
C_BOLD="\033[1m"
C_GREEN="\033[32m"
C_RED="\033[31m"
C_CYAN="\033[36m"
C_YELLOW="\033[33m"

cleanup() { [ -n "$RESP" ] && rm -f "$RESP"; }
trap cleanup EXIT

# Token helper resolution
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/lib/token.sh" ]; then
  source "$SCRIPT_DIR/lib/token.sh"
else
  b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
  token() {
    local user="$1"
    local ttl="${2:-86400}"
    local secret="${3:-${JWT_SECRET:-one-core-challenge-secret}}"
    local header payload sig
    header=$(printf '{"alg":"HS256","typ":"JWT"}' | b64url)
    payload=$(printf '{"sub":"%s","exp":%d}' "$user" $(( $(date +%s) + ttl )) | b64url)
    sig=$(printf '%s.%s' "$header" "$payload" | openssl dgst -sha256 -hmac "$secret" -binary | b64url)
    printf '%s.%s.%s' "$header" "$payload" "$sig"
  }
fi

banner() {
  printf "\n%b▶ %s%b\n" "$C_CYAN$C_BOLD" "$1" "$C_RESET"
}

pass() {
  printf "  %b✔ PASS%b  %s\n" "$C_GREEN" "$C_RESET" "$1"
}

fail() {
  printf "  %b✖ FAIL%b  %s (%s)\n" "$C_RED" "$C_RESET" "$1" "$2"
  FAILS=$((FAILS + 1))
}

warn() {
  printf "  %b! SKIP%b  %s\n" "$C_YELLOW" "$C_RESET" "$1"
}

# HTTP wrapper: executes curl, records HTTP status in CODE, response in $RESP
req() {
  CODE="$(curl -s -o "$RESP" -w '%{http_code}' "$@")"
}

assert_status() {
  local expected="$1"
  local desc="$2"
  if [ "$CODE" = "$expected" ]; then
    pass "$desc (HTTP $expected)"
  else
    fail "$desc" "expected HTTP $expected, got $CODE: $(cat "$RESP" | head -c 120)"
  fi
}

assert_json() {
  local filter="$1"
  local desc="$2"
  if jq -e "$filter" "$RESP" >/dev/null 2>&1; then
    pass "$desc"
  else
    fail "$desc" "JSON assertion failed for '$filter'. Body: $(cat "$RESP" | head -c 150)"
  fi
}

AUTH_U1="Authorization: Bearer $(token "$U1")"
AUTH_U2="Authorization: Bearer $(token "$U2")"

printf "%b====================================================%b\n" "$C_BOLD" "$C_RESET"
printf "%b       1 Core Challenge - Spec Conformance Suite    %b\n" "$C_BOLD" "$C_RESET"
printf "%b====================================================%b\n" "$C_BOLD" "$C_RESET"
printf "Target: %s\n" "$BASE"

# 1. Feed
banner "1. Feed Specification (GET /feed)"
req "$BASE/feed"
assert_status "200" "Endpoint reaches successfully"
assert_json '.posts | type=="array"' "Returns valid 'posts' array"

# 2. Lazy User Materialization
banner "2. User Lifecycle (GET /users/:username before writes)"
req "$BASE/users/$U1"
assert_status "404" "Unseen user returns 404"

# 3. Create Post
banner "3. Post Creation (POST /posts)"
req -X POST -H 'Content-Type: application/json' -H "$AUTH_U1" \
  -d '{"content":"spec check benchmark post"}' "$BASE/posts"
assert_status "201" "Authorized post returns 201"
assert_json '.post.id and .post.username=="'$U1'" and .post.likes==0 and .post.comments==0' \
  "Post schema valid with wrapped 'post' envelope"
PID="$(jq -r '.post.id' "$RESP")"

# 4. Read Post
banner "4. Post Retrieval (GET /posts/:id)"
req "$BASE/posts/$PID"
assert_status "200" "Retrieval returns 200"
assert_json ".post.id==$PID and .post.username==\"$U1\" and .post.likes==0 and .post.comments==0" \
  "Post payload matches created state"

# 5. Likes & Idempotency
banner "5. Like Mechanics & Idempotency (POST /posts/:id/like)"
req -X POST -H "$AUTH_U1" "$BASE/posts/$PID/like"
assert_status "201" "First like from user returns 201"
assert_json ".post_id==$PID and .likes==1" "Likes count increments to 1"

req -X POST -H "$AUTH_U1" "$BASE/posts/$PID/like"
assert_status "200" "Repeat like from same user returns 200 (idempotent)"
assert_json ".post_id==$PID and .likes==1" "Likes count remains unchanged at 1"

req -X POST -H "$AUTH_U2" "$BASE/posts/$PID/like"
assert_status "201" "First like from second user returns 201"
assert_json ".post_id==$PID and .likes==2" "Likes count increments to 2"

# 6. Read Visibility
banner "6. Write Visibility (GET /posts/:id)"
req "$BASE/posts/$PID"
assert_status "200" "Retrieval returns 200"
assert_json '.post.likes==2 and .post.comments==0' "Read reflects committed likes"

# 7. Comments
banner "7. Comment Creation & Listing (POST & GET /posts/:id/comments)"
req -X POST -H 'Content-Type: application/json' -H "$AUTH_U2" \
  -d '{"content":"benchmark comment"}' "$BASE/posts/$PID/comments"
assert_status "201" "Comment creation returns 201"
assert_json ".comment.post_id==$PID and .comment.username==\"$U2\" and .comment.content==\"benchmark comment\"" \
  "Comment schema conforms to wrapped 'comment' envelope"

req "$BASE/posts/$PID/comments"
assert_status "200" "Comments listing returns 200"
assert_json '.comments | type=="array" and length==1 and .[0].username=="'$U2'"' \
  "Comments collection reflects new comment"

# 8. User Profile & Visit Counter
banner "8. User Profiles & Profile Visits (GET /users/:username)"
req "$BASE/users/$U1"
assert_status "200" "Author profile returns 200"
assert_json ".user.username==\"$U1\" and .user.post_count==1 and .user.profile_visits==1" \
  "Profile initial visit increments to 1"

req "$BASE/users/$U1"
assert_status "200" "Repeated profile view returns 200"
assert_json ".user.profile_visits==2" "Profile visit increments reliably to 2"

# 9. Strict Error Handling & Auth Rejection
banner "9. Route, Body & JWT Validation"
req -X POST -H 'Content-Type: application/json' -d '{"content":"no auth"}' "$BASE/posts"
assert_status "401" "Missing Authorization header returns 401"

EXPIRED_TOKEN="$(token "$U1" -3600)"
req -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $EXPIRED_TOKEN" \
  -d '{"content":"expired"}' "$BASE/posts"
assert_status "401" "Expired token returns 401"

WRONG_SECRET_TOKEN="$(token "$U1" 86400 "wrong-secret")"
req -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $WRONG_SECRET_TOKEN" \
  -d '{"content":"bad secret"}' "$BASE/posts"
assert_status "401" "Invalid signature returns 401"

req -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer invalid.token.payload" \
  -d '{"content":"junk"}' "$BASE/posts"
assert_status "401" "Malformed token string returns 401"

req -X POST -H 'Content-Type: application/json' -H "$AUTH_U1" -d '{}' "$BASE/posts"
assert_status "400" "Empty JSON body returns 400"

req -X POST -H 'Content-Type: application/json' -H "$AUTH_U1" -d '{"content":"   "}' "$BASE/posts"
assert_status "400" "Whitespace-only content returns 400"

req "$BASE/undefined-route"
assert_status "404" "Non-existent path returns 404"

req "$BASE/posts/999999999"
assert_status "404" "Non-existent post returns 404"

req -X POST "$BASE/feed"
assert_status "405" "Wrong HTTP method on known route returns 405"

# 10. Crash Durability Spot Check
banner "10. SQLite Crash Durability (kill -9)"
if [ -n "$CONTAINER" ] && docker inspect "$CONTAINER" >/dev/null 2>&1; then
  printf "  Sending SIGKILL to container '%s'...\n" "$CONTAINER"
  docker kill -s 9 "$CONTAINER" >/dev/null 2>&1 || true

  for i in $(seq 1 30); do
    [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" = "false" ] && break
    sleep 1
  done

  docker start "$CONTAINER" >/dev/null
  for i in $(seq 1 30); do
    curl -fsS -o /dev/null "$BASE/feed" 2>/dev/null && break
    sleep 1
  done

  req "$BASE/posts/$PID"
  assert_status "200" "Post available after reboot"
  assert_json '.post.likes==2 and .post.comments==1' "Post write state persisted across kill -9"

  req "$BASE/users/$U1"
  assert_status "200" "User available after reboot"
  assert_json '.user.profile_visits>=2' "User visits persisted across kill -9"
else
  warn "No docker container provided. Durability crash check skipped."
fi

# Summary
printf "\n%b----------------------------------------------------%b\n" "$C_BOLD" "$C_RESET"
if [ "$FAILS" -eq 0 ]; then
  printf "%bRESULT: ALL CHECKS PASSED%b\n" "$C_GREEN$C_BOLD" "$C_RESET"
  exit 0
else
  printf "%bRESULT: %d FAILURE(S)%b\n" "$C_RED$C_BOLD" "$FAILS" "$C_RESET"
  exit 1
fi
