#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p logs
LOG="logs/e2e_curl_$(date -u +%Y%m%d_%H%M%SZ).log"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$LOG"; }

BASE_AUTH="http://127.0.0.1:8083"
BASE_POST="http://127.0.0.1:8081"
BASE_TIMELINE="http://127.0.0.1:8082"
BASE_FOLLOW="http://127.0.0.1:8085"

TS=$(date -u +%Y%m%d%H%M%S)
RND=$((RANDOM%10000))
TEST_EMAIL1="e2e_user1_${TS}_${RND}@example.com"
TEST_EMAIL2="e2e_user2_${TS}_${RND}@example.com"
TEST_PASS1="password1"
TEST_PASS2="password2"

CREATED_POST_IDS=()

# Helper function
curl_and_log() {
  local method="$1"; shift
  local url="$1"; shift
  local data=""
  local -a headers=()
  while (("$#")); do
    case "$1" in
      -d) data="$2"; shift 2;;
      -H) headers+=("$2"); shift 2;;
      *) shift;;
    esac
  done

  log "REQUEST: $method $url"
  for h in "${headers[@]}"; do log "  Header: $h"; done
  [ -n "$data" ] && log "  Data: $data"

  cmd=(curl -s -S -X "$method")
  for h in "${headers[@]}"; do cmd+=(-H "$h"); done
  [ -n "$data" ] && cmd+=(-d "$data")
  cmd+=(-w "\n__HTTP_STATUS__%{http_code}\n" "$url")

  resp="$("${cmd[@]}" 2>&1)"
  status=$(echo "$resp" | tr -d '\r' | sed -n 's/.*__HTTP_STATUS__\([0-9][0-9][0-9]\).*/\1/p' || true)
  body=$(echo "$resp" | sed -n '1,/__HTTP_STATUS__/p' | sed '$d' || true)

  log "STATUS: ${status:-ERR}"
  log "BODY:"
  log "$body"
  log "---"

  echo "$status|$body"
}

log "=== Start demo.sh ==="

# 1) Register users
log "Registering test users"
curl_and_log POST "$BASE_AUTH/register" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL1\",\"password\":\"$TEST_PASS1\"}" || true
curl_and_log POST "$BASE_AUTH/register" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL2\",\"password\":\"$TEST_PASS2\"}" || true

# 2) Login users
log "Logging in"
login1=$(curl -s -X POST "$BASE_AUTH/login" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL1\",\"password\":\"$TEST_PASS1\"}")
login2=$(curl -s -X POST "$BASE_AUTH/login" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL2\",\"password\":\"$TEST_PASS2\"}")
TOKEN1=$(echo "$login1" | jq -r '.token // ""')
TOKEN2=$(echo "$login2" | jq -r '.token // ""')

if [ -z "$TOKEN1" ] || [ -z "$TOKEN2" ]; then
  log "Failed to get tokens! Exiting."
  exit 1
fi
log "Obtained tokens (masked): ${TOKEN1:0:10}..., ${TOKEN2:0:10}..."

# 3) Parse user IDs
parse_user_id() {
  python3 - "$1" <<PY
import sys,json,base64
t=sys.argv[1]
payload=t.split('.')[1]
payload += '=' * (-len(payload) % 4)
data = json.loads(base64.urlsafe_b64decode(payload.encode()).decode())
print(data.get('user_id') or data.get('id') or '')
PY
}

USER1_ID=$(parse_user_id "$TOKEN1")
USER2_ID=$(parse_user_id "$TOKEN2")
if [ -z "$USER1_ID" ] || [ -z "$USER2_ID" ]; then
  log "Failed to parse user IDs from tokens!"
  exit 1
fi
log "Parsed user IDs: A=$USER1_ID B=$USER2_ID"

# 4) user2 follows user1
log "User2 ($USER2_ID) follows User1 ($USER1_ID)"
curl_and_log POST "$BASE_FOLLOW/follow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\":$USER1_ID}"

# 5) user1 creates a post
log "User1 creates a post"
post_resp=$(curl -s -X POST "$BASE_POST/posts" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN1" -d "{\"content\":\"Hello from User1 after follow\"}")
post_id=$(echo "$post_resp" | jq -r '.ID // .id // empty')
CREATED_POST_IDS+=("$post_id")
log "Created post ID: $post_id"

# 6) Wait for fanout to update timeline
log "Checking User2 timeline for post"
found=0
for i in $(seq 1 10); do
  tl=$(curl -s -G "$BASE_TIMELINE/timeline" -H "Authorization: Bearer $TOKEN2")
  log "Timeline attempt $i: $tl"
  if echo "$tl" | jq -e --arg pid "$post_id" '.posts[]? | select((.ID? // .id?) == ($pid|tonumber))' >/dev/null 2>&1; then
    found=1; break
  fi
  sleep 1
done
[ "$found" -eq 1 ] && log "Timeline contains the post" || log "Timeline does NOT contain the post"

# 7) user2 unfollows user1
log "User2 unfollows User1"
curl_and_log POST "$BASE_FOLLOW/unfollow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\":$USER1_ID}"

log "=== Demo complete ==="
