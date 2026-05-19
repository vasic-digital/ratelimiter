#!/usr/bin/env bash
# round-279 — RateLimiter describe + paired-mutation Challenge wrapper.
#
# This wrapper satisfies the parent HelixCode anti-bluff cascade (CONST-035,
# CONST-048, CONST-050, Article XI §11.9). It:
#
#   1. Runs the REAL Challenge runner (challenges/runner/main.go) which
#      exercises the in-memory limiter + HTTP middleware against real
#      httptest sockets with 5-locale bilingual fixtures.
#      Expected exit: 0.
#   2. Runs the PAIRED-MUTATION variant — same binary with
#      RATELIMITER_MUTATE=1 which bypasses the limiter entirely. The
#      assertion in main.go MUST fire on observed allow/deny counts not
#      matching the configured Rate/Burst, causing exit 1. This wrapper
#      translates that to exit 99 (the paired-mutation expected exit
#      code per the cascade convention) so a passing run proves the
#      challenge has teeth.
#
# Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17):
#
#   "all existing tests and Challenges do work in anti-bluff manner -
#    they MUST confirm that all tested codebase really works as expected!
#    We had been in position that all tests do execute with success and
#    all Challenges as well, but in reality the most of the features
#    does not work and can't be used! This MUST NOT be the case and
#    execution of tests and Challenges MUST guarantee the quality, the
#    completion and full usability by end users of the product!"
#
# Usage:
#   bash challenges/ratelimiter_describe_challenge.sh                 # real run, expect exit 0
#   RATELIMITER_MUTATE=1 bash challenges/ratelimiter_describe_challenge.sh   # mutate, expect exit 99

set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MODE="${1:-${MODE:-all}}"

run_real() {
  echo "=== round-279: REAL Challenge run ==="
  if go run ./challenges/runner ; then
    echo "PASS — real run exit 0"
    return 0
  else
    echo "FAIL — real run did not exit 0"
    return 1
  fi
}

run_mutate() {
  echo
  echo "=== round-279: PAIRED-MUTATION Challenge run (expected exit 99) ==="
  set +e
  RATELIMITER_MUTATE=1 go run ./challenges/runner
  rc=$?
  set -e
  echo "mutation runner raw exit: ${rc}"
  if [[ "${rc}" -eq 0 ]]; then
    echo "ASSERTION VIOLATED: mutation variant returned 0 — challenge has no teeth."
    return 2
  fi
  echo "PASS — mutation variant failed as required; translating to canonical exit 99"
  return 99
}

case "$MODE" in
  real)
    run_real
    ;;
  mutate)
    rc=0
    run_mutate || rc=$?
    exit "${rc}"
    ;;
  all|*)
    run_real
    mrc=0
    run_mutate || mrc=$?
    if [[ "${mrc}" -ne 99 ]]; then
      echo "Wrapper detected mutation-variant anomaly (raw=${mrc}); failing closed."
      exit 1
    fi
    echo
    echo "=== round-279 SUMMARY: real=PASS mutation=PROVED (exit 99) ==="
    exit 0
    ;;
esac
