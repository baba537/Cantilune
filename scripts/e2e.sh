#!/usr/bin/env bash
# End-to-end test against a real Navidrome server.
#
# Usage: scripts/e2e.sh <navidrome-version> [path/to/cantilune.ndp]
# Requires: curl, ffmpeg, jq, python3 (Linux amd64).
#
# The test downloads Navidrome, generates a small tagged music library,
# installs the plugin, configures it with the Navidrome CLI and checks the
# generated playlists through the Subsonic API. It also checks that Navidrome
# keeps answering while playlists are generated, exports the configuration
# and verifies the cleanup option.
set -euo pipefail

ND_VERSION="${1:?usage: e2e.sh <navidrome-version> [cantilune.ndp]}"
NDP="$(realpath "${2:-dist/cantilune.ndp}")"
WORK="$(mktemp -d)"
PORT=4533
BASE="http://127.0.0.1:${PORT}"
USER_NAME=admin
PASSWORD=cantilune-e2e
AUTH="u=${USER_NAME}&p=enc:$(printf '%s' "$PASSWORD" | xxd -p | tr -d '\n')&v=1.16.1&c=e2e&f=json"
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/null}"
ND_PID=""

log() { printf '\n==> %s\n' "$*"; }
fail() {
  printf '\nFAIL: %s\n' "$*" >&2
  if [[ -f "$WORK/navidrome.log" ]]; then
    echo "--- last 80 lines of the Navidrome log ---" >&2
    tail -n 80 "$WORK/navidrome.log" >&2
  fi
  exit 1
}
cleanup() {
  [[ -n "$ND_PID" ]] && kill "$ND_PID" 2>/dev/null || true
}
trap cleanup EXIT

export ND_MUSICFOLDER="$WORK/music"
export ND_DATAFOLDER="$WORK/data"
export ND_PLUGINS_ENABLED=true
export ND_PLUGINS_FOLDER="$WORK/data/plugins"
export ND_PORT="$PORT"
export ND_ADDRESS=127.0.0.1
export ND_LOGLEVEL=info
export ND_ENABLEINSIGHTSCOLLECTOR=false

api() { curl -fsS "${BASE}/rest/$1?${AUTH}${2:+&$2}"; }

start_navidrome() {
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
}

log "Downloading Navidrome ${ND_VERSION}"
mkdir -p "$WORK/bin" "$WORK/music" "$ND_PLUGINS_FOLDER"
curl -fsSL "https://github.com/navidrome/navidrome/releases/download/v${ND_VERSION}/navidrome_${ND_VERSION}_linux_amd64.tar.gz" | tar -xz -C "$WORK/bin"

log "Generating test library"
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

cp "$NDP" "$ND_PLUGINS_FOLDER/cantilune.ndp"

log "First start: create admin user and scan the library"
start_navidrome
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "{\"username\":\"${USER_NAME}\",\"password\":\"${PASSWORD}\"}" "${BASE}/auth/createAdmin" >/dev/null
api startScan >/dev/null || true
for _ in $(seq 1 120); do
  status="$(api getScanStatus)"
  if [[ "$(jq -r '.["subsonic-response"].scanStatus.scanning' <<<"$status")" == "false" ]] &&
     [[ "$(jq -r '.["subsonic-response"].scanStatus.count' <<<"$status")" -ge 90 ]]; then
    break
  fi
  sleep 1
done
songs="$(api getScanStatus | jq -r '.["subsonic-response"].scanStatus.count')"
[[ "$songs" -ge 90 ]] || fail "library scan incomplete: $songs songs"
stop_navidrome

log "Configuring the plugin with the Navidrome CLI"
"$WORK/bin/navidrome" plugin validate "$ND_PLUGINS_FOLDER/cantilune.ndp"
CONFIG="$(jq -nc '{
  trackCount: 10,
  logDetails: true,
  gym: {enabled: true, preset: "Hardstyle ⚡"},
  lernen: {enabled: true, preset: "Classical 🎻"},
  autoFahren: {enabled: true, preset: "HipHop 🎤"},
  kochen: {enabled: false}, essen: {enabled: false}, putzen: {enabled: false},
  fokus: {enabled: false}, entspannen: {enabled: false}, schlafen: {enabled: false},
  party: {enabled: false}
}')"
"$WORK/bin/navidrome" plugin edit cantilune --all-users --config "$CONFIG"
"$WORK/bin/navidrome" plugin enable cantilune

log "Second start: generate playlists"
start_navidrome
started="$(date +%s)"
max_ping_ms=0
found=""
for _ in $(seq 1 120); do
  ping_ms="$(curl -fsS -o /dev/null -w '%{time_total}' "${BASE}/rest/ping?${AUTH}" | python3 -c 'import sys; print(int(float(sys.stdin.read())*1000))')"
  (( ping_ms > max_ping_ms )) && max_ping_ms=$ping_ms
  if grep -q "generation finished" "$WORK/navidrome.log"; then
    found=yes
    break
  fi
  sleep 1
done
[[ -n "$found" ]] || fail "plugin did not finish generation"
elapsed=$(( $(date +%s) - started ))

playlists="$(api getPlaylists)"
check_playlist() { # name allowed-genre-regex
  local name="$1" allowed="$2" id entries count bad
  id="$(jq -r --arg n "$name" '.["subsonic-response"].playlists.playlist[] | select(.name == $n) | .id' <<<"$playlists")"
  [[ -n "$id" ]] || fail "playlist '$name' missing. Found: $(jq -c '[.["subsonic-response"].playlists.playlist[].name]' <<<"$playlists")"
  entries="$(api getPlaylist "id=${id}")"
  count="$(jq '.["subsonic-response"].playlist.entry | length' <<<"$entries")"
  [[ "$count" -ge 5 && "$count" -le 10 ]] || fail "'$name' has $count songs"
  bad="$(jq -r --arg re "$allowed" '[.["subsonic-response"].playlist.entry[] | select((.genre // "") | test($re) | not) | .genre] | unique | join(", ")' <<<"$entries")"
  [[ -z "$bad" ]] || fail "'$name' contains unexpected genres: $bad"
  jq -e '.["subsonic-response"].playlist.public == true' <<<"$entries" >/dev/null || fail "'$name' is not public"
  jq -e '.["subsonic-response"].playlist.comment | contains("#cl:")' <<<"$entries" >/dev/null || fail "'$name' has no Cantilune marker"
  echo "ok: $name ($count songs)"
}
check_playlist "🎧 Gym Hardstyle ⚡" '^(Hardstyle|Euphoric Hardstyle)$'
check_playlist "🎧 Studying Classical 🎻" '^Classical$'
check_playlist "🎧 Driving HipHop 🎤" '^Hip-Hop$'
grep -q "for admin #1: " "$WORK/navidrome.log" || fail "song explanations missing in the log"
generation_line="$(grep -o 'generation finished[^"]*' "$WORK/navidrome.log" | tail -n 1)"
stop_navidrome

log "Exporting the configuration"
"$WORK/bin/navidrome" plugin info cantilune --format json >"$WORK/info.json"
grep -q "Hardstyle" "$WORK/info.json" || fail "configuration export does not contain the settings"

log "Cleanup option removes all playlists"
"$WORK/bin/navidrome" plugin edit cantilune --config "$(jq -c '. + {removeAllPlaylists: true}' <<<"$CONFIG")"
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
  echo "- Library: ${songs} generated songs"
  echo "- 3 playlists generated and verified, cleanup verified"
  echo "- Time from start to finished generation: ${elapsed}s (includes the 15 s startup delay)"
  echo "- Slowest ping during generation: ${max_ping_ms} ms"
  echo "- Plugin log: \`${generation_line}\`"
} | tee -a "$SUMMARY"
