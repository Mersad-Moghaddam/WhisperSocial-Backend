#!/usr/bin/env bash
set -euo pipefail

# Lightweight bash load tester for TS-timeline-system
# - No external build tools required (only curl, jq, sleep)
# - Launches N concurrent "virtual users" (background jobs) performing:
#     register -> login -> create post -> fetch timeline
# - Aggregates successes and reports error rate
# - Ramps concurrency from START to MAX by STEP and stops when error rate > THRESHOLD
# - Logs detailed output to logs/loadtest_bash_<ts>.log

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

LOGDIR="logs"
mkdir -p "$LOGDIR"
LOGFILE="$LOGDIR/loadtest_bash_$(date -u +%Y%m%d_%H%M%SZ).log"

AUTH_URL="http://127.0.0.1:8083"
POST_URL="http://127.0.0.1:8081"
TIMELINE_URL="http://127.0.0.1:8082"
FOLLOW_URL="http://127.0.0.1:8085"

START=10
STEP=10
MAX=200
THRESHOLD=0.05
TIMEOUT=5

usage(){
  cat <<EOF
Usage: $0 [--start N] [--step N] [--max N] [--threshold F] [--timeout S]
Defaults: start=$START step=$STEP max=$MAX threshold=$THRESHOLD timeout=$TIMEOUT
Example: $0 --start 20 --step 20 --max 200 --threshold 0.05
EOF
}

# parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --start) START="$2"; shift 2;;
    --step) STEP="$2"; shift 2;;
    --max) MAX="$2"; shift 2;;
    --threshold) THRESHOLD="$2"; shift 2;;
    --timeout) TIMEOUT="$2"; shift 2;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown arg: $1"; usage; exit 1;;
  esac
done

for cmd in curl jq; do
  if ! command -v $cmd >/dev/null 2>&1; then
    echo "Required command not found: $cmd" >&2
    exit 1
  fi
done

log(){
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$LOGFILE"
}

# perform one virtual user sequence
# args: <index> <round_id> <tmpdir>
vu_sequence(){
  local idx="$1"; shift
  local runid="$1"; shift
  local tmpdir="$1"; shift

  local ts=$(date +%s)
  local rand=$RANDOM
  local email1="lt_u_${runid}_${idx}_${ts}_${rand}@example.com"
  local pass="passw0rd"

  # register (ignore 409)
  http_status=$(curl -s -S -o "$tmpdir/reg_${idx}.out" -w "%{http_code}" --max-time $TIMEOUT -X POST "$AUTH_URL/register" -H "Content-Type: application/json" -d "{\"email\":\"$email1\",\"password\":\"$pass\"}")
  echo "$http_status" > "$tmpdir/reg_status_${idx}"

  # login
  login_resp=$(curl -s -S --max-time $TIMEOUT -X POST "$AUTH_URL/login" -H "Content-Type: application/json" -d "{\"email\":\"$email1\",\"password\":\"$pass\"}") || true
  token=$(echo "$login_resp" | jq -r '.token // empty')
  if [ -z "$token" ]; then
    echo "FAIL" > "$tmpdir/result_${idx}"
    return
  fi

  # create post
  post_resp=$(curl -s -S --max-time $TIMEOUT -X POST "$POST_URL/posts" -H "Content-Type: application/json" -H "Authorization: Bearer $token" -d '{"content":"loadtest post"}') || true
  post_id=$(echo "$post_resp" | jq -r '.ID // .id // empty')
  if [ -z "$post_id" ]; then
    echo "FAIL" > "$tmpdir/result_${idx}"
    return
  fi

  # get timeline
  tl_resp=$(curl -s -S --max-time $TIMEOUT -G "$TIMELINE_URL/timeline" -H "Authorization: Bearer $token") || true
  # success if timeline call returns JSON with posts array (status check via jq)
  if echo "$tl_resp" | jq -e '.posts' >/dev/null 2>&1; then
    echo "SUCCESS" > "$tmpdir/result_${idx}"
  else
    echo "FAIL" > "$tmpdir/result_${idx}"
  fi
}

log "Starting bash loadtest: start=$START step=$STEP max=$MAX threshold=$THRESHOLD timeout=$TIMEOUT"
prev_good=0
runid=$(date +%s)

for conc in $(seq $START $STEP $MAX); do
  log "Running concurrency level: $conc"
  tmpdir=$(mktemp -d)
  # spawn VUs
  for i in $(seq 1 $conc); do
    vu_sequence "$i" "$runid" "$tmpdir" &
  done
  wait

  total=$conc
  success=$(grep -c "^SUCCESS$" "$tmpdir"/result_* 2>/dev/null || true)
  fail=$((total - success))
  err_rate=0
  if [ $total -gt 0 ]; then
    err_rate=$(awk "BEGIN {printf \"%.4f\", ($fail)/($total)}")
  fi
  log "concurrency=$conc total=$total success=$success fail=$fail err_rate=$err_rate"

  # Save detailed per-run results into log
  log "Detailed results (first 20 entries):"
  head -n 20 "$tmpdir"/result_* 2>/dev/null | tee -a "$LOGFILE"

  if awk "BEGIN {exit !($err_rate > $THRESHOLD)}"; then
    log "Error rate $err_rate exceeded threshold $THRESHOLD. Stopping. Max good concurrency ~= $prev_good"
    # cleanup
    rm -rf "$tmpdir"
    exit 0
  fi

  prev_good=$conc
  rm -rf "$tmpdir"
  # small sleep between rounds
  sleep 1
done

log "Completed ramp to max=$MAX. Estimated max_good_concurrency=$prev_good"
exit 0
