#!/usr/bin/env bash
#
# Drives the dockerised gateway stack and asserts the behaviours the gateway
# promises: a model is listed once running, a cold request wakes a node and
# streams a reply, the engine key is injected and never reaches the caller, a
# wrong token is 401, and wake: off refuses. This is both the CI integration
# test and something a maintainer can run locally — there is no CI-only path
# that can drift from what you run by hand.
#
# Usage: ./run-tests.sh [--keep]
#   --keep  leave the stack running afterwards, to poke at it yourself

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly HERE
REPO_ROOT="$(cd "${HERE}/../.." && pwd)"
readonly REPO_ROOT
# Where the built binary lands; the stack is driven by the spinloop built from
# this working tree, so the test covers this commit.
readonly SPINLOOP_BIN="${HERE}/.spinloop-test-bin"
readonly READY_TIMEOUT_SECS=90
readonly GATEWAY_URL=http://127.0.0.1:4000
readonly GATEWAY_COLD_URL=http://127.0.0.1:4001
readonly COMPLETION_BODY='{"model":"fake-model","messages":[{"role":"user","content":"hi"}]}'

keep_stack=0
failures=0

#######################################
# Report a passing assertion.
# Arguments:
#   Description of what passed.
# Outputs:
#   Writes the result to stdout.
#######################################
pass() {
  echo "  ok   - $1"
}

#######################################
# Report a failing assertion and record it, without aborting the run — one
# failure should not hide the rest.
# Globals:
#   failures
# Arguments:
#   Description, expected, actual.
# Outputs:
#   Writes the failure to stderr.
#######################################
fail() {
  echo "  FAIL - $1" >&2
  echo "         expected: $2" >&2
  echo "         actual:   $3" >&2
  failures=$((failures + 1))
}

#######################################
# Assert that a string contains a substring.
# Arguments:
#   Description, haystack, needle.
#######################################
assert_contains() {
  local description="$1" haystack="$2" needle="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    pass "${description}"
  else
    fail "${description}" "to contain '${needle}'" "${haystack}"
  fi
}

#######################################
# Assert that a string does not contain a substring.
# Arguments:
#   Description, haystack, needle.
#######################################
assert_not_contains() {
  local description="$1" haystack="$2" needle="$3"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    pass "${description}"
  else
    fail "${description}" "not to contain '${needle}'" "${haystack}"
  fi
}

#######################################
# Assert an exact string equality.
# Arguments:
#   Description, actual, expected.
#######################################
assert_equals() {
  local description="$1" actual="$2" expected="$3"
  if [[ "${actual}" == "${expected}" ]]; then
    pass "${description}"
  else
    fail "${description}" "${expected}" "${actual}"
  fi
}

#######################################
# Run a docker compose command, showing its output only when it fails. These
# commands are noisy on success and the test's own output is the point, but a
# silent failure is worse than noise: a `compose up` that cannot pull leaves
# nothing behind but "Tearing down..." and an exit code.
# Globals:
#   HERE
# Arguments:
#   Arguments to pass to docker compose.
# Returns:
#   The command's exit status.
#######################################
compose() {
  local out rc=0
  out="$(docker compose -f "${HERE}/compose.yaml" "$@" 2>&1)" || rc=$?
  if (( rc != 0 )); then
    echo "Error: docker compose $* failed (exit ${rc}):" >&2
    echo "${out}" >&2
  fi
  return "${rc}"
}

#######################################
# The container state for one service, or "" when docker cannot say.
# Globals:
#   HERE
# Arguments:
#   Service name.
# Outputs:
#   Writes the state to stdout.
#######################################
container_state() {
  docker compose -f "${HERE}/compose.yaml" ps --format '{{.State}}' "$1" 2>/dev/null
}

#######################################
# Dump what the containers are doing, for a wait that timed out.
# Globals:
#   HERE
# Outputs:
#   Writes container state and recent logs to stderr.
#######################################
diagnose_fleet() {
  docker compose -f "${HERE}/compose.yaml" ps >&2 2>&1 || true
  docker compose -f "${HERE}/compose.yaml" logs --tail 20 >&2 2>&1 || true
}

#######################################
# Run `spinloop fleet` against the example's fleet.yaml.
# Globals:
#   SPINLOOP_BIN, HERE
# Arguments:
#   Arguments to pass to `spinloop fleet`.
# Outputs:
#   The command's stdout; stderr is discarded so assertions read cleanly.
#######################################
fleet() {
  "${SPINLOOP_BIN}" fleet "$@" --fleet "${HERE}/fleet.yaml" 2>/dev/null
}

#######################################
# As fleet(), but merging stderr — for assertions about error messages.
# Globals:
#   SPINLOOP_BIN, HERE
#######################################
fleet_with_stderr() {
  "${SPINLOOP_BIN}" fleet "$@" --fleet "${HERE}/fleet.yaml" 2>&1
}

#######################################
# The state column for one node, or "" when the node is absent.
# Arguments:
#   Node name.
#######################################
node_state() {
  local name="$1"
  fleet status | awk -v n="${name}" '$1 == n {print $2}'
}

#######################################
# The gateway container's logs, for assertions about what it did.
# Globals:
#   HERE
#######################################
gateway_logs() {
  docker compose -f "${HERE}/compose.yaml" logs --no-color gateway 2>/dev/null || true
}

#######################################
# The process list inside node-a.
# Globals:
#   HERE
#######################################
node_a_processes() {
  docker compose -f "${HERE}/compose.yaml" exec -T node-a ps ax 2>/dev/null || true
}

#######################################
# GET a gateway path with the gateway's token.
# Globals:
#   GATEWAY_URL, GATEWAY_TOKEN
# Arguments:
#   Port (4000 or 4001), path.
#######################################
gateway_get() {
  local port="$1" path="$2"
  curl -fsS -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
    "http://127.0.0.1:${port}${path}" 2>/dev/null || true
}

#######################################
# POST a completion body at a gateway. Returns the body; a non-2xx makes the
# function fail, which is what the positive-path assertions want.
# Globals:
#   GATEWAY_TOKEN
# Arguments:
#   Port, path, body, extra curl arguments (e.g. a header dump file flag).
#######################################
gateway_post() {
  local port="$1" path="$2" body="$3"
  shift 3
  curl -fsS -X POST -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
    -H "Content-Type: application/json" -d "${body}" \
    "http://127.0.0.1:${port}${path}" "$@" 2>/dev/null || true
}

#######################################
# Wait until both node daemons answer, so assertions do not race the
# containers' startup.
# Globals:
#   READY_TIMEOUT_SECS
# Returns:
#   0 once both nodes report a state, 1 on timeout.
#######################################
wait_for_fleet() {
  local deadline=$((SECONDS + READY_TIMEOUT_SECS))
  while (( SECONDS < deadline )); do
    # Read the table into a variable rather than piping it: under `pipefail`
    # a `grep -q` that matches and exits first can leave the pipeline
    # reporting the writer's SIGPIPE, which reads here as "nothing
    # unreachable" — the opposite of what was found.
    if [[ "$(fleet status)" != *unreachable* ]]; then
      return 0
    fi
    sleep 2
  done
  echo "Error: the fleet did not become reachable in ${READY_TIMEOUT_SECS}s" >&2
  fleet status >&2 || true
  diagnose_fleet
  return 1
}

#######################################
# Wait until both gateways answer /health.
# Globals:
#   READY_TIMEOUT_SECS, GATEWAY_TOKEN
#######################################
wait_for_gateways() {
  local deadline=$((SECONDS + READY_TIMEOUT_SECS))
  while (( SECONDS < deadline )); do
    if [[ -n "$(gateway_get 4000 /health)" && -n "$(gateway_get 4001 /health)" ]]; then
      return 0
    fi
    sleep 2
  done
  echo "Error: the gateways did not come up in ${READY_TIMEOUT_SECS}s" >&2
  diagnose_fleet
  return 1
}

#######################################
# Wait for one node to reach a state.
# Arguments:
#   Node name, expected state, timeout in seconds.
# Returns:
#   0 when the state is reached, 1 on timeout.
#######################################
wait_for_state() {
  local name="$1" want="$2" timeout="$3"
  local deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    if [[ "$(node_state "${name}")" == "${want}" ]]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

#######################################
# Tear the stack down unless --keep was given. Registered as an EXIT trap so a
# failure part-way through still cleans up.
# Globals:
#   keep_stack, HERE
#######################################
cleanup() {
  if (( keep_stack )); then
    echo
    echo "Stack left running (--keep). Try:"
    echo "  cd ${HERE} && set -a && . ./.env && set +a"
    echo "  curl -H 'Authorization: Bearer \$GATEWAY_TOKEN' http://127.0.0.1:4000/v1/models"
    echo "Tear down with: docker compose -f ${HERE}/compose.yaml down -v"
    return
  fi
  echo
  echo "Tearing down..."
  docker compose -f "${HERE}/compose.yaml" down -v >/dev/null 2>&1 || true
  rm -f "${SPINLOOP_BIN}"
}

#######################################
# Assert the gateway's own door: its token in, 401 out, and a 404 that names
# what it serves.
#######################################
test_gateway_auth() {
  echo "The gateway's own door"
  local ok
  ok="$(gateway_get 4000 /health)"
  assert_contains "health answers with the token" "${ok}" '"ok":true'
  assert_equals "no token is 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' "${GATEWAY_URL}/health")" "401"
  assert_equals "a wrong token is 401" \
    "$(curl -s -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer not-the-token' "${GATEWAY_URL}/health")" "401"

  local out
  out="$(curl -s -w '\n%{http_code}' -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
    "${GATEWAY_URL}/v1/nope" 2>/dev/null || true)"
  assert_contains "an unknown path names the paths served" "${out}" "the gateway serves"
  assert_contains "it names /v1/models" "${out}" "/v1/models"
  assert_contains "and it is a 404" "${out}" "404"
}

#######################################
# Assert a cold fleet lists what a wake can start, and the wake: off gateway
# lists nothing — it decides whether to start, and nothing is running.
#######################################
test_cold_listing() {
  echo "A cold fleet lists what a wake can start"
  local models cold
  models="$(gateway_get 4000 /v1/models)"
  assert_contains "the waking gateway lists the source's served name" \
    "${models}" '"id":"fake-model"'
  cold="$(gateway_get 4001 /v1/models)"
  assert_contains "the wake: off gateway lists nothing, since nothing runs" \
    "${cold}" '"data":[]'
}

#######################################
# Assert the wake: off gateway refuses a cold request, names the node and the
# command that would start it, and starts nothing.
#######################################
test_wake_off_refuses_cold() {
  echo "wake: off refuses a cold request"
  local out
  out="$(curl -s -w '\n%{http_code}' -X POST \
    -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
    -H "Content-Type: application/json" -d "${COMPLETION_BODY}" \
    "${GATEWAY_COLD_URL}/v1/chat/completions" 2>/dev/null || true)"
  assert_contains "nothing is serving, so the request fails" "${out}" "503"
  assert_contains "the failure names the policy" "${out}" "wake is off"
  assert_contains "it names the node and the start command" \
    "${out}" "spinloop fleet start node-a"
  assert_equals "and node-a was not started" "$(node_state node-a)" "idle"
  assert_equals "nor was node-b" "$(node_state node-b)" "idle"
}

#######################################
# Assert the command the refusal names actually starts the node.
#######################################
test_suggested_start_works() {
  echo "The command the refusal names starts the node"
  fleet start node-a >/dev/null
  if wait_for_state node-a running 30; then
    pass "fleet start node-a brings it up"
  else
    fail "fleet start node-a brings it up" "running" "$(node_state node-a)"
  fi
  assert_contains "status shows what it serves" "$(fleet status)" "fake-model"
}

#######################################
# Assert wake: off still routes to what is already running: it decides
# whether to start, not whether to answer.
#######################################
test_wake_off_routes_running() {
  echo "wake: off still routes what is already running"
  # The cold gateway's last reading is from before the start; let it go stale
  # rather than race the two-second cache.
  sleep 3
  local out
  out="$(gateway_post 4001 /v1/chat/completions "${COMPLETION_BODY}")"
  assert_contains "the running node answers through the cold gateway" \
    "${out}" "Hello from the fake engine"
}

#######################################
# Assert the model list is what the fleet is running.
#######################################
test_models_listing() {
  echo "The model list is what the fleet is running"
  local models
  models="$(gateway_get 4000 /v1/models)"
  assert_contains "the gateway lists the served name" "${models}" '"id":"fake-model"'
}

#######################################
# Assert a running node answers through the waking gateway, and the gateway
# logged the route with its node and its wake state.
#######################################
test_gateway_serves_running_node() {
  echo "A running node answers through the gateway"
  local out
  out="$(gateway_post 4000 /v1/chat/completions "${COMPLETION_BODY}")"
  assert_contains "the reply is the engine's" "${out}" "Hello from the fake engine"

  local logs
  logs="$(gateway_logs)"
  assert_contains "the gateway logged the route" "${logs}" "msg=routed"
  assert_contains "it named the node" "${logs}" "node=node-a"
  assert_contains "it knew the node was already running" "${logs}" "woken=false"
}

#######################################
# Assert the engine key is injected and never reaches the caller: the engine
# is gated (a direct call without the key is refused, with it answered), the
# key arrived as a file path, and no reply, log or process list carries it.
#######################################
test_engine_key_gating() {
  echo "The engine key is injected, and never reaches the caller"
  local url="http://127.0.0.1:18080/v1/chat/completions"
  assert_equals "a direct call with no key is refused" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
       -d "${COMPLETION_BODY}" "${url}")" "401"
  assert_equals "a direct call with a wrong key is refused" \
    "$(curl -s -o /dev/null -w '%{http_code}' -X POST \
       -H "Authorization: Bearer not-the-key" -H 'Content-Type: application/json' \
       -d "${COMPLETION_BODY}" "${url}")" "401"
  local reply
  reply="$(curl -fsS -X POST -H "Authorization: Bearer ${NODE_A_ENGINE_KEY}" \
    -H 'Content-Type: application/json' -d "${COMPLETION_BODY}" "${url}" 2>/dev/null || true)"
  assert_contains "the node's own key opens it" "${reply}" "Hello from the fake engine"

  local status
  status="$(curl -fsS -H "Authorization: Bearer ${NODE_A_TOKEN}" \
    http://127.0.0.1:14242/v1/status 2>/dev/null || true)"
  assert_contains "the node reports its engine needs a key" "${status}" '"requiresKey":true'
  assert_not_contains "the node never discloses the key" "${status}" "${NODE_A_ENGINE_KEY}"

  local enginelog
  enginelog="$(fleet logs node-a --limit 50 2>/dev/null || true)"
  assert_contains "the engine was gated by file" "${enginelog}" "--api-key-file"
  assert_not_contains "the key itself never reaches the command line" \
    "${enginelog}" "${NODE_A_ENGINE_KEY}"
  assert_not_contains "the key is not in the node's process list" \
    "$(node_a_processes)" "${NODE_A_ENGINE_KEY}"

  local via_gateway
  via_gateway="$(gateway_post 4000 /v1/chat/completions "${COMPLETION_BODY}")"
  assert_not_contains "the key never appears in a reply through the gateway" \
    "${via_gateway}" "${NODE_A_ENGINE_KEY}"
}

#######################################
# Assert a cold request to the waking gateway starts a node and the streamed
# reply passes through in the server-sent-events shape.
#######################################
test_cold_request_wakes_and_streams() {
  echo "A cold request wakes a node and streams a reply"
  fleet stop node-a >/dev/null
  wait_for_state node-a stopped 30 || true
  # Let the gateway's reading of the fleet go stale, so the request below
  # sees the stop rather than a two-second-old reading of a running node.
  sleep 3

  local headers="${HERE}/.stream-headers"
  local out
  out="$(gateway_post 4000 /v1/chat/completions \
    '{"model":"fake-model","stream":true,"messages":[{"role":"user","content":"hi"}]}' \
    -D "${headers}" || true)"
  assert_contains "a streamed reply is server-sent-events" \
    "$(cat "${headers}" 2>/dev/null)" "text/event-stream"
  assert_contains "the chunks pass through" "${out}" "Hello from the fake engine"
  assert_contains "the stream ends" "${out}" "data: [DONE]"
  rm -f "${headers}"

  if wait_for_state node-a running 60; then
    pass "the request left the node it woke running"
  else
    fail "the request left the node it woke running" "running" "$(node_state node-a)"
  fi
  assert_contains "the gateway logged the wake" \
    "$(gateway_logs)" "Waking node-a to serve fake-model"
}

#######################################
# Assert a harness launch against the client's Spinloop points the agent at
# the gateway's address with the gateway's token as its key.
# Globals:
#   HERE, SPINLOOP_BIN
#######################################
test_launch_points_agent_at_gateway() {
  echo "A launch points the agent at the gateway"
  local sandbox="${HERE}/.launch-sandbox"
  rm -rf "${sandbox}"
  mkdir -p "${sandbox}/bin" "${sandbox}/home"
  cat > "${sandbox}/bin/opencode" <<'STUB'
#!/usr/bin/env bash
echo "HARNESS base_url=${OPENAI_BASE_URL:-<unset>} key=${OPENAI_API_KEY:-<unset>}"
STUB
  chmod +x "${sandbox}/bin/opencode"

  local launch
  launch="$(PATH="${sandbox}/bin:${PATH}" HOME="${sandbox}/home" \
    XDG_CONFIG_HOME="${sandbox}/home/.config" \
    OPENAI_BASE_URL="" \
    OPENAI_API_KEY="${GATEWAY_TOKEN}" \
    "${SPINLOOP_BIN}" harness -O="${HERE}/client/Spinloop" -H opencode 2>&1 || true)"
  assert_contains "the agent is pointed at the gateway with its prefix" \
    "${launch}" "base_url=http://127.0.0.1:4000/v1"
  assert_contains "the agent is given the gateway's token as its key" \
    "${launch}" "key=${GATEWAY_TOKEN}"
  local config="${sandbox}/home/.config/opencode/opencode.json"
  if [[ -f "${config}" ]]; then
    pass "the harness config was written"
    assert_contains "and it carries the gateway's address" \
      "$(cat "${config}")" "127.0.0.1:4000"
  else
    fail "the harness config was written" "${config}" "missing"
  fi
  rm -rf "${sandbox}"
}

#######################################
# Assert a launch that cannot authenticate the gateway fails before the agent
# is started and before anything is written, naming the variable.
# Globals:
#   HERE, SPINLOOP_BIN
#######################################
test_launch_fails_without_token() {
  echo "A launch without the gateway's token fails, naming the variable"
  local sandbox="${HERE}/.launch-sandbox"
  rm -rf "${sandbox}"
  mkdir -p "${sandbox}/bin" "${sandbox}/home"
  cat > "${sandbox}/bin/opencode" <<'STUB'
#!/usr/bin/env bash
echo "HARNESS base_url=${OPENAI_BASE_URL:-<unset>} key=${OPENAI_API_KEY:-<unset>}"
STUB
  chmod +x "${sandbox}/bin/opencode"

  local launch
  launch="$(PATH="${sandbox}/bin:${PATH}" HOME="${sandbox}/home" \
    XDG_CONFIG_HOME="${sandbox}/home/.config" \
    OPENAI_BASE_URL="" \
    OPENAI_API_KEY="" \
    "${SPINLOOP_BIN}" harness -O="${HERE}/client/Spinloop" -H opencode 2>&1 || true)"
  assert_contains "the failure names the variable to set" "${launch}" "OPENAI_API_KEY"
  assert_not_contains "the agent was not started" "${launch}" "HARNESS"
  if [[ ! -f "${sandbox}/home/.config/opencode/opencode.json" ]]; then
    pass "no harness config was written"
  else
    fail "no harness config was written" "no opencode.json" "one was written"
  fi
  rm -rf "${sandbox}"
}

main() {
  if [[ "${1:-}" == "--keep" ]]; then
    keep_stack=1
  fi

  cd "${HERE}"
  if [[ ! -f .env ]]; then
    echo "Using .env.example for tokens (no .env present)"
    cp .env.example .env
  fi
  set -a
  # shellcheck source=/dev/null
  . ./.env
  set +a

  trap cleanup EXIT

  echo "Building spinloop from the working tree..."
  (cd "${REPO_ROOT}" && go build -o "${SPINLOOP_BIN}" ./cmd/spinloop)

  echo "Bringing the stack up..."
  compose up -d --build
  wait_for_fleet
  wait_for_gateways

  echo
  test_gateway_auth
  echo
  test_cold_listing
  echo
  test_wake_off_refuses_cold
  echo
  test_suggested_start_works
  echo
  test_wake_off_routes_running
  echo
  test_models_listing
  echo
  test_gateway_serves_running_node
  echo
  test_engine_key_gating
  echo
  test_cold_request_wakes_and_streams
  echo
  test_launch_points_agent_at_gateway
  echo
  test_launch_fails_without_token

  echo
  if (( failures > 0 )); then
    echo "${failures} assertion(s) failed" >&2
    return 1
  fi
  echo "All assertions passed"
}

main "$@"
