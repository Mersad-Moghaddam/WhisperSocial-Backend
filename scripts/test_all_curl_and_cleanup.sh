#!/usr/bin/env bash
set -euo pipefail

# Full curl test + cleanup script
# - Runs API checks for all services and logs results to logs/e2e_curl_<ts>.log
# - Cleans up created test data (posts, followers, users, timeline entries) using DB and Redis

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p logs
LOG="logs/e2e_curl_$(date -u +%Y%m%d_%H%M%SZ).log"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$LOG"; }

# Load .env for DB/REDIS credentials (must exist at repo root)
if [ -f .env ]; then
  # shellcheck disable=SC1091
  source .env
else
  log "No .env file found at $ROOT_DIR/.env — DB/Redis cleanup may not work"
fi

# Endpoints
BASE_AUTH="http://127.0.0.1:8083"
BASE_POST="http://127.0.0.1:8081"
BASE_TIMELINE="http://127.0.0.1:8082"
BASE_FOLLOW="http://127.0.0.1:8085"

# Test accounts (unique per run)
TS=$(date -u +%Y%m%d%H%M%S)
RND=$((RANDOM%10000))
TEST_EMAIL1="e2e_user1_${TS}_${RND}@example.com"
TEST_EMAIL2="e2e_user2_${TS}_${RND}@example.com"
TEST_PASS1="password1"
TEST_PASS2="password2"

CREATED_POST_IDS=()

# Helper: run curl and log request/response
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

  # Build curl command
  cmd=(curl -s -S -X "$method")
  for h in "${headers[@]}"; do
    cmd+=(-H "$h")
  done
  if [ -n "$data" ]; then
    cmd+=(-d "$data")
  fi
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

######### Run tests #########

log "=== Start full curl test ==="

# 1) Register users (OK if they already exist)
log "Registering test users (ignore errors if they already exist)"
curl_and_log POST "$BASE_AUTH/register" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL1\",\"password\":\"$TEST_PASS1\"}" || true
curl_and_log POST "$BASE_AUTH/register" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL2\",\"password\":\"$TEST_PASS2\"}" || true

# 2) Login users
log "Logging in test users"
login1=$(curl -s -X POST "$BASE_AUTH/login" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL1\",\"password\":\"$TEST_PASS1\"}")
login2=$(curl -s -X POST "$BASE_AUTH/login" -H "Content-Type: application/json" -d "{\"email\":\"$TEST_EMAIL2\",\"password\":\"$TEST_PASS2\"}")
TOKEN1=$(echo "$login1" | jq -r '.token // ""')
TOKEN2=$(echo "$login2" | jq -r '.token // ""')

if [ -z "$TOKEN1" ] || [ -z "$TOKEN2" ]; then
  log "Failed to obtain tokens; aborting tests"
  exit 1
fi
log "Obtained tokens (masked): ${TOKEN1:0:10}..., ${TOKEN2:0:10}..."

# parse user_id from token payload via python
parse_user_id(){
  python3 - "$1" <<PY
import sys,json,base64
t=sys.argv[1]
payload=t.split('.')[1]
payload += '=' * (-len(payload) % 4)
print(json.loads(base64.urlsafe_b64decode(payload.encode()).decode()).get('user_id',''))
PY
}
USER1_ID=$(parse_user_id "$TOKEN1")
USER2_ID=$(parse_user_id "$TOKEN2")
log "Parsed user IDs: user1=$USER1_ID user2=$USER2_ID"

# 3) user2 follows user1
log "User2 ($USER2_ID) follows User1 ($USER1_ID)"
curl_and_log POST "$BASE_FOLLOW/follow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\":$USER1_ID}"

# 4) user1 creates a post
log "User1 creates a post"
post_resp=$(curl -s -X POST "$BASE_POST/posts" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN1" -d "{\"content\":\"E2E test post\"}")
post_id=$(echo "$post_resp" | jq -r '.ID // .id // empty')
if [ -n "$post_id" ]; then
  CREATED_POST_IDS+=("$post_id")
  log "Created post ID: $post_id"
else
  log "Failed to get created post ID"; log "$post_resp"
fi

# 5) Wait for fanout and verify timeline
log "Waiting for fanout to deliver post to user2 timeline"
found=0
for i in $(seq 1 10); do
  tl=$(curl -s -G "$BASE_TIMELINE/timeline" -H "Authorization: Bearer $TOKEN2")
  log "Timeline attempt $i: $tl"
  if echo "$tl" | jq -e --arg pid "$post_id" '.posts[]? | select((.ID? // .id?) == ($pid|tonumber))' >/dev/null 2>&1; then
    found=1; break
  fi
  sleep 1
done
if [ "$found" -eq 1 ]; then log "Timeline contains the post"; else log "Timeline does NOT contain the post"; fi

# 6) user2 unfollows user1 via API
log "User2 unfollows User1"
curl_and_log POST "$BASE_FOLLOW/unfollow" -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN2" -d "{\"user_id\":$USER1_ID}"

######### Cleanup #########
log "=== Cleanup: removing test data ==="

# Remove timeline zsets in Redis for both users
if [ -n "$REDIS_ADDR" ]; then
  log "Cleaning Redis timelines"
  # keys timeline:<id>
  redis-cli -h ${REDIS_ADDR%%:*} -p ${REDIS_ADDR##*:} KEYS "timeline:*" | while read -r k; do
    if echo "$k" | grep -q "timeline:"; then
      # only remove timeline entries for our test users if present
      if echo "$k" | grep -E "timeline:($USER1_ID|$USER2_ID)" >/dev/null 2>&1; then
        log "DEL $k"
        redis-cli -h ${REDIS_ADDR%%:*} -p ${REDIS_ADDR##*:} DEL "$k"
      fi
    fi
  done || true
  # Optionally trim stream entries (leave stream alone but acknowledge pending)
fi

# Delete posts and users from MySQL (requires mysql client and .env DB_* vars)
if command -v mysql >/dev/null 2>&1 && [ -n "${DB_USER-}" ] && [ -n "${DB_NAME-}" ]; then
  log "Cleaning MySQL entries (posts, followers, users)"
  # Remove posts created by USER1 and USER2
  for pid in "${CREATED_POST_IDS[@]}"; do
    log "DELETE FROM posts WHERE id=$pid"
    mysql -u"$DB_USER" -h "${DB_HOST%%:*}" -P "${DB_HOST##*:}" "$DB_NAME" -e "DELETE FROM posts WHERE id=$pid;"
  done
  # Remove followers rows where user_id or follower_id matches our users
  mysql -u"$DB_USER" -h "${DB_HOST%%:*}" -P "${DB_HOST##*:}" "$DB_NAME" -e "DELETE FROM followers WHERE user_id IN ($USER1_ID,$USER2_ID) OR follower_id IN ($USER1_ID,$USER2_ID);"
  # Remove test users by email
  mysql -u"$DB_USER" -h "${DB_HOST%%:*}" -P "${DB_HOST##*:}" "$DB_NAME" -e "DELETE FROM users WHERE email IN ('$TEST_EMAIL1','$TEST_EMAIL2');"
else
  log "MySQL client or DB_* env vars missing; skipping DB cleanup"
fi

log "Cleanup completed. Log is at $LOG"
