#!/usr/bin/env bash
# 1 Core Challenge - spec check
# Verifies an entry implements the full API (api.md), then runs the
# kill -9 durability spot check (rules.md).
#
# Usage:   ./spec-check.sh [container-name] [base-url]
# Example: ./spec-check.sh challenge http://localhost:8080
# Needs:   curl, jq, and docker when a container name is given.
set -uo pipefail

CONTAINER="${1:-}"
BASE="${2:-http://localhost:8080}"
FAILS=0
RESP="$(mktemp)"
TS="$(date +%s)"
U="tc_${TS}"   # throwaway username, unique per run

cleanup() { [ -n "$RESP" ] && rm -f "$RESP"; }
trap cleanup EXIT

code() { curl -s -o "$RESP" -w '%{http_code}' "$@"; }
body() { cat "$RESP"; }

ok()   { echo "PASS - $1"; }
bad()  { echo "FAIL - $1"; FAILS=$((FAILS+1)); }
expect_code() { # want got label
  if [ "$2" = "$1" ]; then ok "$3"; else bad "$3 (expected $1, got $2)"; fi
}
expect_json() { # jq filter, label
  if echo "$(body)" | jq -e "$1" >/dev/null 2>&1; then ok "$2"; else bad "$2 (json did not match: $(body | head -c 200))"; fi
}

echo "== 1. GET /feed =="
C="$(code "$BASE/feed")"
expect_code 200 "$C" "GET /feed returns 200"
expect_json '.posts | type=="array"' "feed.posts is an array (empty feed is ok)"

echo "== 2. POST /posts =="
C="$(code -X POST -H 'Content-Type: application/json' \
  -d "{\"username\":\"$U\",\"content\":\"spec check post\"}" "$BASE/posts")"
expect_code 201 "$C" "POST /posts returns 201"
PID="$(echo "$(body)" | jq -r '.id')"
expect_json '.likes==0 and .comments==0' "new post starts with zero likes and comments"

echo "== 3. GET /posts/:id =="
C="$(code "$BASE/posts/$PID")"
expect_code 200 "$C" "GET /posts/:id returns 200"
expect_json ".id==$PID and .likes==0 and .comments==0" "post carries like and comment counts"

echo "== 4. POST /posts/:id/like =="
C="$(code -X POST "$BASE/posts/$PID/like")"
expect_code 201 "$C" "like returns 201"
expect_json ".likes==1" "first like makes total 1"
C="$(code -X POST "$BASE/posts/$PID/like")"
expect_json ".likes==2" "second like makes total 2 (double liking is allowed)"

echo "== 5. write visible to read =="
C="$(code "$BASE/posts/$PID")"
expect_json '.likes==2 and .comments==0' "GET /posts shows the like that just got a 201"

echo "== 6. POST /posts/:id/comments =="
C="$(code -X POST -H 'Content-Type: application/json' \
  -d "{\"username\":\"$U\",\"content\":\"spec check comment\"}" "$BASE/posts/$PID/comments")"
expect_code 201 "$C" "comment returns 201"

echo "== 7. GET /posts/:id/comments =="
C="$(code "$BASE/posts/$PID/comments")"
expect_code 200 "$C" "comments list returns 200"
expect_json '.comments | length==1' "comment shows up in the list"

echo "== 8. GET /users/:username =="
C="$(code "$BASE/users/$U")"
expect_code 200 "$C" "profile returns 200"
expect_json ".username==\"$U\" and .post_count==1" "profile carries username and post_count"
C="$(code "$BASE/users/$U")"
expect_json '.profile_visits>=2' "second profile view increments profile_visits"

echo "== 9. errors =="
C="$(code "$BASE/no-such-path")"
expect_code 404 "$C" "unknown path returns 404"
C="$(code "$BASE/posts/999999999")"
expect_code 404 "$C" "unknown post returns 404"
C="$(code -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/posts")"
expect_code 400 "$C" "empty body returns 400"
C="$(code "$BASE/posts")"
expect_code 405 "$C" "GET /posts returns 405"

echo "== 10. kill -9 durability (rules.md spot check) =="
if [ -n "$CONTAINER" ] && docker inspect "$CONTAINER" >/dev/null 2>&1; then
  docker exec "$CONTAINER" sh -c 'kill -9 1' >/dev/null 2>&1 || true
  for i in $(seq 1 30); do
    [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" = "false" ] && break
    sleep 1
  done
  docker start "$CONTAINER" >/dev/null
  for i in $(seq 1 30); do
    curl -fsS -o /dev/null "$BASE/feed" && break
    sleep 1
  done
  C="$(code "$BASE/posts/$PID")"
  expect_json '.likes==2 and .comments==1' "likes and comments survived kill -9"
else
  echo "SKIP - no docker container given. Run it with one:  ./spec-check.sh challenge"
fi

echo
if [ "$FAILS" -eq 0 ]; then
  echo "SPEC CHECK PASSED"
else
  echo "SPEC CHECK FAILED ($FAILS failure(s))"
  exit 1
fi
