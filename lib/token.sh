#!/usr/bin/env bash
# Makes HS256 JWTs for the shell scripts (spec-check.sh).
# Usage:
#   source lib/token.sh
#   token user0001                 # valid for 24 hours, secret from JWT_SECRET
#   token user0001 -3600           # already expired (negative seconds)
#   token user0001 86400 other     # signed with a different secret

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
