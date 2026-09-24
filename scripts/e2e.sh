#!/usr/bin/env bash
# End-to-end test against a real Navidrome server.
#
# Usage: scripts/e2e.sh <navidrome-version> [cantilune.ndp] [previous.ndp]
# Requires: curl, ffmpeg, jq, python3 (Linux amd64).
#
# The test downloads Navidrome, generates a tagged music library, installs the
# plugin, configures it with the Navidrome CLI and checks the generated
# playlists through the Subsonic API. It also checks that Navidrome keeps
# answering while playlists are generated, exports the configuration and
# verifies the cleanup option.
#
# With previous.ndp (for example the latest release), that version is
# installed and configured first and then replaced by cantilune.ndp, the way
# users update. The test then checks that the settings are kept, the plugin
# stays enabled and every playlist exists exactly once.
#
# E2E_EXTRA_SONGS (default 1500) adds songs of unrelated genres so that the
# measured generation time reflects a library of realistic size.
set -euo pipefail

ND_VERSION="${1:?usage: e2e.sh <navidrome-version> [cantilune.ndp] [previous.ndp]}"
NDP="$(realpath "${2:-dist/cantilune.ndp}")"
PREVIOUS_NDP="${3:+$(realpath "$3")}"
EXTRA_SONGS="${E2E_EXTRA_SONGS:-1500}"
WORK="$(mktemp -d)"
PORT=4533
BASE="http://127.0.0.1:${PORT}"
USER_NAME=admin
PASSWORD=cantilune-e2e
AUTH="u=${USER_NAME}&p=enc:$(printf '%s' "$PASSWORD" | xxd -p | tr -d '\n')&v=1.16.1&c=e2e&f=json"
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/null}"
ND_PID=""
max_ping_ms=0

log() { printf '\n==> %s\n' "$*"; }
fail() {
  trap - ERR
  printf '\nFAIL: %s\n' "$*" >&2
  local tail_log=""
  if [[ -f "$WORK/navidrome.log" ]]; then
    echo "--- last 80 lines of the Navidrome log ---" >&2
    tail -n 80 "$WORK/navidrome.log" >&2
    tail_log="$(grep -E 'level=(error|fatal)|plugin=cantilune|[Pp]lugin|Scan' "$WORK/navidrome.log" | tail -n 20 | cut -c1-300 | sed ':a;N;$!ba;s/\n/%0A/g')"
  fi
  if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
    echo "::error title=E2E Navidrome ${ND_VERSION}::$*"
    [[ -n "$tail_log" ]] && echo "::notice title=Navidrome ${ND_VERSION} log::${tail_log}"
  fi
  exit 1
}
cleanup() {
  [[ -n "$ND_PID" ]] && kill "$ND_PID" 2>/dev/null || true
}
trap cleanup EXIT
trap 'fail "command failed (line $LINENO): $BASH_COMMAND"' ERR

export ND_MUSICFOLDER="$WORK/music"
export ND_DATAFOLDER="$WORK/data"
export ND_PLUGINS_ENABLED=true
export ND_PLUGINS_FOLDER="$WORK/data/plugins"
export ND_PORT="$PORT"
export ND_ADDRESS=127.0.0.1
export ND_LOGLEVEL=info
export ND_ENABLEINSIGHTSCOLLECTOR=false
# No artist lookups on external services: keeps the log readable and the test offline.
export ND_ENABLEEXTERNALSERVICES=false

api() { curl -fsS "${BASE}/rest/$1?${AUTH}${2:+&$2}"; }
navidrome() { "$WORK/bin/navidrome" "$@"; }

start_navidrome() {
  ! curl -fsS "${BASE}/ping" >/dev/null 2>&1 || fail "another server is already listening on port ${PORT}"
  # Started directly (not through the navidrome function) so that $! is Navidrome's PID.
  "$WORK/bin/navidrome" >>"$WORK/navidrome.log" 2>&1 &
  ND_PID=$!
  for _ in $(seq 1 60); do
    curl -fsS "${BASE}/ping" >/dev/null 2>&1 && return 0
    sleep 1
  done
  fail "Navidrome did not start"
}

stop_navidrome() {
  kill "$ND_PID"
  wait "$ND_PID" 2>/dev/null || true
  ND_PID=""
  for _ in $(seq 1 30); do
    curl -fsS "${BASE}/ping" >/dev/null 2>&1 || return 0
    sleep 1
  done
  fail "Navidrome did not stop"
}

generation_runs() { grep -c "generation finished" "$WORK/navidrome.log" || true; }

# wait_generation <n>: waits until the plugin reported n finished runs, measuring
# Navidrome's response time meanwhile.
wait_generation() {
  local want="$1" ping_ms
  for _ in $(seq 1 180); do
    ping_ms="$(curl -fsS -o /dev/null -w '%{time_total}' "${BASE}/rest/ping?${AUTH}" | python3 -c 'import sys; print(int(float(sys.stdin.read())*1000))')"
    (( ping_ms > max_ping_ms )) && max_ping_ms=$ping_ms
    (( $(generation_runs) >= want )) && return 0
    sleep 1
  done
  fail "plugin did not finish generation (expected run $want)"
}

last_generation_line() { grep -o 'generation finished[^"]*' "$WORK/navidrome.log" | tail -n 1; }

check_playlist() { # name allowed-genre-regex
  local name="$1" allowed="$2" playlists id entries count bad
  playlists="$(api getPlaylists)"
  id="$(jq -r --arg n "$name" '[.["subsonic-response"].playlists.playlist[] | select(.name == $n) | .id] | join(" ")' <<<"$playlists")"
  [[ -n "$id" ]] || fail "playlist '$name' missing. Found: $(jq -c '[.["subsonic-response"].playlists.playlist[].name]' <<<"$playlists")"
  [[ "$id" != *" "* ]] || fail "playlist '$name' exists more than once"
  entries="$(api getPlaylist "id=${id}")"
  count="$(jq '.["subsonic-response"].playlist.entry | length' <<<"$entries")"
  [[ "$count" -ge 5 && "$count" -le 10 ]] || fail "'$name' has $count songs"
  bad="$(jq -r --arg re "$allowed" '[.["subsonic-response"].playlist.entry[] | select((.genre // "") | test($re) | not) | .genre] | unique | join(", ")' <<<"$entries")"
  [[ -z "$bad" ]] || fail "'$name' contains unexpected genres: $bad"
  jq -e '.["subsonic-response"].playlist.public == true' <<<"$entries" >/dev/null || fail "'$name' is not public"
  jq -e '.["subsonic-response"].playlist.comment | contains("#cl:")' <<<"$entries" >/dev/null || fail "'$name' has no Cantilune marker"
  echo "ok: $name ($count songs)"
}

check_all_playlists() {
  local managed
  check_playlist "🎧 Gym Hardstyle ⚡" '^(Hardstyle|Euphoric Hardstyle)$'
  check_playlist "🎧 Studying Classical 🎻" '^Classical$'
  check_playlist "🎧 Driving HipHop 🎤" '^Hip-Hop$'
  managed="$(api getPlaylists | jq '[.["subsonic-response"].playlists.playlist[]? | select(.comment // "" | contains("#cl:"))] | length')"
  [[ "$managed" == "3" ]] || fail "expected 3 Cantilune playlists, found $managed"
}

log "Downloading Navidrome ${ND_VERSION}"
mkdir -p "$WORK/bin" "$WORK/music" "$ND_PLUGINS_FOLDER"
curl -fsSL "https://github.com/navidrome/navidrome/releases/download/v${ND_VERSION}/navidrome_${ND_VERSION}_linux_amd64.tar.gz" | tar -xz -C "$WORK/bin"

log "Generating test library (90 tagged songs + ${EXTRA_SONGS} of other genres)"
generate() { # genre count first-year
  local genre="$1" count="$2" year="$3"
  for i in $(seq 1 "$count"); do
    ffmpeg -loglevel error -f lavfi -i "sine=frequency=$((180 + i * 11)):duration=95" -ac 1 -b:a 32k \
      -metadata title="${genre} Song ${i}" -metadata artist="${genre} Artist $((i % 6))" \
      -metadata album="${genre} Album" -metadata genre="${genre}" -metadata date="$((year + i % 20))" \
      "$WORK/music/${genre// /_}_${i}.mp3"
  done
}
generate "Hardstyle" 24 2005
generate "Euphoric Hardstyle" 8 2012
generate "Hip-Hop" 20 1990
generate "Classical" 20 1960
generate "Dancehall" 12 2000
generate "Audiobook" 6 2010

# Songs of unrelated genres, copied from one template to keep this fast.
ffmpeg -loglevel error -f lavfi -i "sine=frequency=440:duration=150" -ac 1 -b:a 32k "$WORK/template.mp3"
filler_genres=(Rock Pop Jazz Electronic Metal Folk Soul Reggae Country Ambient)
for i in $(seq 1 "$EXTRA_SONGS"); do
  genre="${filler_genres[$((i % ${#filler_genres[@]}))]}"
  ffmpeg -loglevel error -i "$WORK/template.mp3" -c copy \
    -metadata title="${genre} Track ${i}" -metadata artist="${genre} Band $((i % 150))" \
    -metadata album="${genre} Album $((i % 300))" -metadata genre="${genre}" -metadata date="$((1970 + i % 55))" \
    "$WORK/music/filler_${i}.mp3"
done
expected_songs=$((90 + EXTRA_SONGS))

cp "${PREVIOUS_NDP:-$NDP}" "$ND_PLUGINS_FOLDER/cantilune.ndp"

log "First start: create admin user and scan the library"
start_navidrome
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "{\"username\":\"${USER_NAME}\",\"password\":\"${PASSWORD}\"}" "${BASE}/auth/createAdmin" >/dev/null
api startScan >/dev/null || true
for _ in $(seq 1 300); do
  status="$(api getScanStatus)"
  if [[ "$(jq -r '.["subsonic-response"].scanStatus.scanning' <<<"$status")" == "false" ]] &&
     [[ "$(jq -r '.["subsonic-response"].scanStatus.count' <<<"$status")" -ge "$expected_songs" ]]; then
    break
  fi
  sleep 1
done
songs="$(api getScanStatus | jq -r '.["subsonic-response"].scanStatus.count')"
[[ "$songs" -ge "$expected_songs" ]] || fail "library scan incomplete: $songs of $expected_songs songs"
stop_navidrome

log "Configuring the plugin with the Navidrome CLI"
if navidrome plugin --help 2>&1 | grep -qw validate; then
  navidrome plugin validate "$ND_PLUGINS_FOLDER/cantilune.ndp"
else
  echo "navidrome plugin validate is not available in ${ND_VERSION}, skipping manifest validation"
fi
CONFIG="$(jq -nc '{
  trackCount: 10,
  logDetails: true,
  gym: {enabled: true, style: "Hardstyle ⚡"},
  lernen: {enabled: true, style: "Classical 🎻"},
  autoFahren: {enabled: true, style: "HipHop 🎤"},
  kochen: {enabled: false}, essen: {enabled: false}, putzen: {enabled: false},
  fokus: {enabled: false}, entspannen: {enabled: false}, schlafen: {enabled: false},
  party: {enabled: false}
}')"
navidrome plugin edit cantilune --all-users --config "$CONFIG"
navidrome plugin enable cantilune

log "Second start: generate playlists${PREVIOUS_NDP:+ with the previous version}"
start_navidrome
started="$(date +%s)"
wait_generation 1
elapsed=$(( $(date +%s) - started ))
check_all_playlists
grep -q "for admin #1: " "$WORK/navidrome.log" || fail "song explanations missing in the log"
generation_line="$(last_generation_line)"
stop_navidrome

update_line=""
if [[ -n "$PREVIOUS_NDP" ]]; then
  log "Updating the plugin file to the new version"
  cp "$NDP" "$ND_PLUGINS_FOLDER/cantilune.ndp"
  start_navidrome
  wait_generation 2
  check_all_playlists
  update_line="$(last_generation_line)"
  stop_navidrome
  echo "ok: plugin still enabled after the update, playlists unique (${update_line})"
fi

log "Exporting the configuration"
navidrome plugin info cantilune --format json >"$WORK/info.json"
grep -q "Hardstyle" "$WORK/info.json" || fail "configuration export does not contain the settings"

log "Cleanup option removes all playlists"
navidrome plugin edit cantilune --config "$(jq -c '. + {removeAllPlaylists: true}' <<<"$CONFIG")"
start_navidrome
for _ in $(seq 1 60); do
  grep -q "cleanup finished" "$WORK/navidrome.log" && break
  sleep 1
done
remaining="$(api getPlaylists | jq '[.["subsonic-response"].playlists.playlist[]? | select(.comment // "" | contains("#cl:"))] | length')"
[[ "$remaining" == "0" ]] || fail "$remaining Cantilune playlists left after cleanup"
stop_navidrome

log "Passed"
{
  echo "### Navidrome ${ND_VERSION}"
  echo "- Library: ${songs} songs"
  echo "- 3 playlists generated and verified, cleanup verified"
  [[ -n "$update_line" ]] && echo "- Update from the previous release verified: \`${update_line}\`"
  echo "- Time from start to finished generation: ${elapsed}s (includes the 15 s startup delay)"
  echo "- Slowest ping during generation: ${max_ping_ms} ms"
  echo "- Plugin log: \`${generation_line}\`"
} | tee -a "$SUMMARY"
if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "::notice title=E2E Navidrome ${ND_VERSION} passed::${songs} songs, 3 playlists verified${update_line:+, update from previous release verified}, cleanup verified, slowest ping ${max_ping_ms} ms. ${generation_line}"
fi
