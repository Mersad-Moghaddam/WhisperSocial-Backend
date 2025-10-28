#!/usr/bin/env bash
set -euo pipefail

BASE_URL_AUTH="http://127.0.0.1:8083"
BASE_URL_POST="http://127.0.0.1:8081"
BASE_URL_TIMELINE="http://127.0.0.1:8082"
BASE_URL_FOLLOW="http://127.0.0.1:8085"

tmpdir=$(mktemp -d)
cleanup(){ rm -rf "$tmpdir"; }
trap cleanup EXIT

echo "1) Register two users"
curl -s -X POST "$BASE_URL_AUTH/register" -H "Content-Type: application/json" -d '{"email":"e2e_user1@example.com","password":"password1"}' | jq . || true
curl -s -X POST "$BASE_URL_AUTH/register" -H "Content-Type: application/json" -d '{"email":"e2e_user2@example.com","password":"password2"}' | jq . || true

echo "\n2) Login both users and capture tokens"
curl -s -X POST "$BASE_URL_AUTH/login" -H "Content-Type: application/json" -d '{"email":"e2e_user1@example.com","password":"password1"}' -o "$tmpdir/login1.json"
curl -s -X POST "$BASE_URL_AUTH/login" -H "Content-Type: application/json" -d '{"email":"e2e_user2@example.com","password":"password2"}' -o "$tmpdir/login2.json"
TOKEN1=$(jq -r .token "$tmpdir/login1.json" | tr -d '\n' )
TOKEN2=$(jq -r .token "$tmpdir/login2.json" | tr -d '\n' )
echo "user1 token: ${TOKEN1:0:20}..."
echo "user2 token: ${TOKEN2:0:20}..."

echo "\n3) user2 follows user1"
# The follow endpoint expects {"user_id": <target>}. We need user IDs. Tokens encode user_id; but easier: call follow service /following or use /followers? We'll parse user id from token via jwt.io-like parsing (base64).

# Helper to parse user_id claim (uses python to be portable on macOS/linux)
parse_user_id(){
  token="$1"
  python3 - "$token" <<PY
import sys, json, base64
token = sys.argv[1]
try:
    payload = token.split('.')[1]
    payload += '=' * (-len(payload) % 4)
    data = json.loads(base64.urlsafe_b64decode(payload.encode()).decode())
    print(data.get('user_id',''))
except Exception:
    print('')
PY
}
USER1_ID=$(parse_user_id "$TOKEN1")
USER2_ID=$(parse_user_id "$TOKEN2")
echo "user1 id: $USER1_ID, user2 id: $USER2_ID"

if [ -z "$USER1_ID" ] || [ -z "$USER2_ID" ]; then
  echo "Failed to parse user IDs from tokens; aborting." >&2
  exit 2
fi

curl -s -X POST "$BASE_URL_FOLLOW/follow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\": $USER1_ID}" | jq .

echo "\n4) user1 creates a post"
curl -s -X POST "$BASE_URL_POST/posts" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN1" -d '{"content":"Hello from user1 - e2e"}' -o "$tmpdir/post.json"
cat "$tmpdir/post.json" | jq .
POST_ID=$(jq -r .ID "$tmpdir/post.json")

echo "\nWaiting for fanout worker to deliver the post to user2's timeline (retries)"
MAX_RETRIES=10
SLEEP=0.5
found=0
for i in $(seq 1 $MAX_RETRIES); do
  curl -s -G "$BASE_URL_TIMELINE/timeline" -H "Authorization: Bearer $TOKEN2" -o "$tmpdir/timeline.json"
  if jq -e --argjson pid "$POST_ID" '.posts[]? | select((.ID? // .id?) == $pid)' "$tmpdir/timeline.json" >/dev/null 2>&1; then
    found=1
    break
  fi
  sleep $SLEEP
done

echo "\nTimeline response from last attempt:"
cat "$tmpdir/timeline.json" | jq .

if [ "$found" -eq 1 ]; then
  echo "\n✅ Timeline contains the new post (post ID: $POST_ID)"
else
  echo "\n❌ Timeline does NOT contain the new post after retries; failing." >&2
  exit 2
fi

echo "\n6) user2 unfollows user1"
curl -s -X POST "$BASE_URL_FOLLOW/unfollow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\": $USER1_ID}" | jq .

echo "\nE2E test completed successfully." 
